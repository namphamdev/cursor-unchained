package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/bernardo-bruning/ollama-copilot/internal/proto"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/ollama/ollama/api"
)

// CompletionRequest is the request sent to the completion handler
type CompletionRequest struct {
	Extra struct {
		Language          string `json:"language"`
		NextIndent        int    `json:"next_indent"`
		PromptTokens      int    `json:"prompt_tokens"`
		SuffixTokens      int    `json:"suffix_tokens"`
		TrimByIndentation bool   `json:"trim_by_indentation"`
	} `json:"extra"`
	MaxTokens   int      `json:"max_tokens"`
	N           int      `json:"n"`
	Prompt      string   `json:"prompt"`
	Stop        []string `json:"stop"`
	Stream      bool     `json:"stream"`
	Suffix      string   `json:"suffix"`
	Temperature float64  `json:"temperature"`
	TopP        int      `json:"top_p"`
}

// Logprobs is the logprobs returned by the CompletionResponse
type Logprobs struct {
	Tokens []struct {
		Token   string  `json:"token"`
		Logprob float64 `json:"logprob"`
	} `json:"tokens"`
}

// ChoiceResponse is the response returned CompletionResponse
type ChoiceResponse struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	Logprobs     *Logprobs
	FinishReason string `json:"finish_reason"`
}

// CompletionResponse is the response returned by the CompletionHandler
type CompletionResponse struct {
	Id      string           `json:"id"`
	Created int64            `json:"created"`
	Choices []ChoiceResponse `json:"choices"`
}

// Prompt is an repreentation of a prompt with suffi and prefix
type Prompt struct {
	Prefix string
	Suffix string
}

func (p Prompt) Generate(templ *template.Template) string {
	var buf = new(bytes.Buffer)
	err := templ.Execute(buf, p)
	if err != nil {
		log.Printf("error executing prompt template: %s", err.Error())
	}

	return buf.String()
}

// CompletionHandler is an http.Handler that returns completions.
type CompletionHandler struct {
	api        *api.Client
	model      string
	templ      *template.Template
	numPredict int
}

// StreamCppConfig holds configuration for StreamCpp requests
type StreamCppConfig struct {
	BearerToken   string
	ClientVersion string
	RequestID     string
	SessionID     string
}

// LoadStreamCppConfig loads StreamCpp configuration from environment variables
func LoadStreamCppConfig() (*StreamCppConfig, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	bearerToken := os.Getenv("CURSOR_BEARER_TOKEN")
	if bearerToken == "" {
		return nil, fmt.Errorf("CURSOR_BEARER_TOKEN environment variable is required")
	}

	clientVersion := os.Getenv("X_CURSOR_CLIENT_VERSION")
	if clientVersion == "" {
		clientVersion = "0.50.5" // Default version
	}

	requestID := os.Getenv("X_REQUEST_ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	sessionID := os.Getenv("X_SESSION_ID")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	return &StreamCppConfig{
		BearerToken:   bearerToken,
		ClientVersion: clientVersion,
		RequestID:     requestID,
		SessionID:     sessionID,
	}, nil
}

// StreamCppResult holds the result of a StreamCpp request
type StreamCppResult struct {
	Text       string
	DoneStream bool
	DoneEdit   bool
	Error      error
}

// NewCompletionHandler returns a new CompletionHandler.
func NewCompletionHandler(api *api.Client, model string, template *template.Template, numPredict int) *CompletionHandler {
	return &CompletionHandler{api, model, template, numPredict}
}

// sendStreamCppRequest sends a request to Cursor's StreamCpp endpoint
func sendStreamCppRequest(ctx context.Context, config StreamCppConfig, fileContents string, languageID string, cursorLine int32, cursorColumn int32) (*StreamCppResult, error) {
	// Build the StreamCppRequest protobuf
	req := &proto.StreamCppRequest{
		CurrentFile: &proto.CurrentFileInfo{
			RelativeWorkspacePath: "Untitled-1",
			Contents:              fileContents,
			CursorPosition: &proto.CursorPosition{
				Line:   cursorLine,
				Column: cursorColumn,
			},
			LanguageId:  languageID,
			FileVersion: 2,
			LineEnding:  "\n",
		},
		ModelName: "fast",
		CppIntentInfo: &proto.CppIntentInfo{
			Source: "line_change",
		},
		ClientTime:            float64(time.Now().UnixMilli()),
		TimeSinceRequestStart: 0,
		TimeAtRequestSend:     float64(time.Now().UnixMilli()),
		SupportsCpt:           false,
		SupportsCrlfCpt:       false,
	}

	// Marshal to protobuf using custom Marshal method
	protoData, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	// Gzip compress the data
	var gzipBuf bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipBuf)
	if _, err := gzipWriter.Write(protoData); err != nil {
		return nil, fmt.Errorf("failed to gzip data: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Build Connect protocol envelope: 1 byte flags + 4 bytes length + data
	compressedData := gzipBuf.Bytes()
	envelope := make([]byte, 5+len(compressedData))
	envelope[0] = 0x01 // Compression flag (gzip)
	binary.BigEndian.PutUint32(envelope[1:5], uint32(len(compressedData)))
	copy(envelope[5:], compressedData)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://us-only.gcpp.cursor.sh:443/aiserver.v1.AiService/StreamCpp",
		bytes.NewReader(envelope))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("connect-accept-encoding", "gzip")
	httpReq.Header.Set("connect-content-encoding", "gzip")
	httpReq.Header.Set("connect-protocol-version", "1")
	httpReq.Header.Set("content-type", "application/connect+proto")
	httpReq.Header.Set("x-cursor-client-type", "ide")
	httpReq.Header.Set("x-cursor-client-version", config.ClientVersion)
	httpReq.Header.Set("x-cursor-streaming", "true")
	httpReq.Header.Set("x-request-id", config.RequestID)
	httpReq.Header.Set("x-session-id", config.SessionID)
	httpReq.Header.Set("Authorization", "Bearer "+config.BearerToken)

	// Create HTTP client with TLS config
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	// Send request
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read and parse response stream
	result := &StreamCppResult{}
	var dataBuffer bytes.Buffer

	for {
		// Read envelope header (5 bytes: 1 byte flags + 4 bytes length)
		header := make([]byte, 5)
		_, err := io.ReadFull(resp.Body, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read envelope header: %w", err)
		}

		flags := header[0]
		messageLen := binary.BigEndian.Uint32(header[1:5])

		// Read message data
		messageData := make([]byte, messageLen)
		_, err = io.ReadFull(resp.Body, messageData)
		if err != nil {
			return nil, fmt.Errorf("failed to read message data: %w", err)
		}

		// Check if this is a trailer (flags & 0x02)
		if flags&0x02 != 0 {
			// Trailer is JSON, skip for now
			continue
		}

		// Decompress if gzipped (flags & 0x01)
		var decompressedData []byte
		if flags&0x01 != 0 {
			gzipReader, err := gzip.NewReader(bytes.NewReader(messageData))
			if err != nil {
				return nil, fmt.Errorf("failed to create gzip reader: %w", err)
			}
			decompressedData, err = io.ReadAll(gzipReader)
			gzipReader.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to decompress data: %w", err)
			}
		} else {
			decompressedData = messageData
		}

		// Parse protobuf response using custom Unmarshal method
		respMsg := &proto.StreamCppResponse{}
		if err := respMsg.Unmarshal(decompressedData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Accumulate text
		if respMsg.Text != "" {
			dataBuffer.WriteString(respMsg.Text)
		}

		if respMsg.DoneStream {
			result.DoneStream = true
		}
		if respMsg.DoneEdit {
			result.DoneEdit = true
			break
		}
	}

	result.Text = dataBuffer.String()
	return result, nil
}

// ServeHTTP implements http.Handler.
func (c *CompletionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	req := CompletionRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Fatalf("error decode: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Try StreamCpp first
	config, err := LoadStreamCppConfig()
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second*60)
		defer cancel()

		// Use the prompt as file contents, detect language from request
		languageID := req.Extra.Language
		if languageID == "" {
			languageID = "javascript"
		}

		// Calculate cursor position (end of prompt)
		lines := 0
		lastLineLen := 0
		for _, ch := range req.Prompt {
			if ch == '\n' {
				lines++
				lastLineLen = 0
			} else {
				lastLineLen++
			}
		}

		result, err := sendStreamCppRequest(ctx, *config, req.Prompt, languageID, int32(lines), int32(lastLineLen))
		log.Printf("StreamCpp result: %+v", result)
		if err == nil && result.Text != "" {
			response := CompletionResponse{
				Id:      uuid.New().String(),
				Created: time.Now().Unix(),
				Choices: []ChoiceResponse{
					{
						Text:         result.Text,
						Index:        0,
						FinishReason: "stop",
					},
				},
			}

			_, err := w.Write([]byte("data: "))
			if err != nil {
				log.Printf("error writing response: %v", err)
				return
			}

			if err := json.NewEncoder(w).Encode(response); err != nil {
				log.Printf("error encoding response: %v", err)
				return
			}
			return
		}

		if err != nil {
			log.Printf("StreamCpp error, falling back to Ollama: %v", err)
		}
	}

	// Fallback to Ollama
	generate := api.GenerateRequest{
		Model:  c.model,
		Prompt: Prompt{Prefix: req.Prompt, Suffix: req.Suffix}.Generate(c.templ),
		Options: map[string]interface{}{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
			"stop":        req.Stop,
			"num_predict": c.numPredict,
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*60)
	r = r.WithContext(ctx)
	defer cancel()
	doneChan := make(chan struct{})
	err = c.api.Generate(r.Context(), &generate, func(resp api.GenerateResponse) error {
		response := CompletionResponse{
			Id:      uuid.New().String(),
			Created: time.Now().Unix(),
			Choices: []ChoiceResponse{
				{
					Text:  resp.Response,
					Index: 0,
				},
			},
		}

		_, err := w.Write([]byte("data: "))
		if err != nil {
			cancel()
			return err
		}

		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			cancel()
			return err
		}
		if resp.Done {
			close(doneChan)
		}

		return nil
	})

	if err == nil {
		select {
		case <-r.Context().Done():
			err = r.Context().Err()
		case <-doneChan:
		}
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error generating completion: %v", err)
		return
	}
}
