package proto

import (
	"math"

	"google.golang.org/protobuf/encoding/protowire"
)

// CursorPosition represents cursor location in the file
type CursorPosition struct {
	Line   int32
	Column int32
}

// CurrentFileInfo contains information about the current file being edited
type CurrentFileInfo struct {
	RelativeWorkspacePath string
	Contents              string
	CursorPosition        *CursorPosition
	LanguageId            string
	FileVersion           int32
	LineEnding            string
	WorkspaceRootPath     string
}

// CppIntentInfo contains intent information for cpp request
type CppIntentInfo struct {
	Source string
}

// StreamCppRequest is the request message for StreamCpp
type StreamCppRequest struct {
	CurrentFile           *CurrentFileInfo
	ModelName             string
	CppIntentInfo         *CppIntentInfo
	ClientTime            float64
	TimeSinceRequestStart float64
	TimeAtRequestSend     float64
	SupportsCpt           bool
	SupportsCrlfCpt       bool
}

func floatToFixed64(f float64) uint64 {
	return math.Float64bits(f)
}

// Marshal encodes StreamCppRequest to protobuf bytes
func (r *StreamCppRequest) Marshal() ([]byte, error) {
	var buf []byte

	// Field 1: current_file (message)
	if r.CurrentFile != nil {
		fileData := r.CurrentFile.marshal()
		buf = protowire.AppendTag(buf, 1, protowire.BytesType)
		buf = protowire.AppendBytes(buf, fileData)
	}

	// Field 3: model_name (string)
	if r.ModelName != "" {
		buf = protowire.AppendTag(buf, 3, protowire.BytesType)
		buf = protowire.AppendString(buf, r.ModelName)
	}

	// Field 16: cpp_intent_info (message)
	if r.CppIntentInfo != nil {
		intentData := r.CppIntentInfo.marshal()
		buf = protowire.AppendTag(buf, 16, protowire.BytesType)
		buf = protowire.AppendBytes(buf, intentData)
	}

	// Field 21: client_time (double/fixed64)
	if r.ClientTime != 0 {
		buf = protowire.AppendTag(buf, 21, protowire.Fixed64Type)
		buf = protowire.AppendFixed64(buf, floatToFixed64(r.ClientTime))
	}

	// Field 23: time_since_request_start (double/fixed64)
	buf = protowire.AppendTag(buf, 23, protowire.Fixed64Type)
	buf = protowire.AppendFixed64(buf, floatToFixed64(r.TimeSinceRequestStart))

	// Field 24: time_at_request_send (double/fixed64)
	if r.TimeAtRequestSend != 0 {
		buf = protowire.AppendTag(buf, 24, protowire.Fixed64Type)
		buf = protowire.AppendFixed64(buf, floatToFixed64(r.TimeAtRequestSend))
	}

	// Field 27: supports_cpt (bool)
	buf = protowire.AppendTag(buf, 27, protowire.VarintType)
	if r.SupportsCpt {
		buf = protowire.AppendVarint(buf, 1)
	} else {
		buf = protowire.AppendVarint(buf, 0)
	}

	// Field 28: supports_crlf_cpt (bool)
	buf = protowire.AppendTag(buf, 28, protowire.VarintType)
	if r.SupportsCrlfCpt {
		buf = protowire.AppendVarint(buf, 1)
	} else {
		buf = protowire.AppendVarint(buf, 0)
	}

	return buf, nil
}

func (c *CurrentFileInfo) marshal() []byte {
	var buf []byte

	// Field 1: relative_workspace_path (string)
	if c.RelativeWorkspacePath != "" {
		buf = protowire.AppendTag(buf, 1, protowire.BytesType)
		buf = protowire.AppendString(buf, c.RelativeWorkspacePath)
	}

	// Field 2: contents (string)
	if c.Contents != "" {
		buf = protowire.AppendTag(buf, 2, protowire.BytesType)
		buf = protowire.AppendString(buf, c.Contents)
	}

	// Field 3: cursor_position (message)
	if c.CursorPosition != nil {
		posData := c.CursorPosition.marshal()
		buf = protowire.AppendTag(buf, 3, protowire.BytesType)
		buf = protowire.AppendBytes(buf, posData)
	}

	// Field 5: language_id (string)
	if c.LanguageId != "" {
		buf = protowire.AppendTag(buf, 5, protowire.BytesType)
		buf = protowire.AppendString(buf, c.LanguageId)
	}

	// Field 14: file_version (int32)
	if c.FileVersion != 0 {
		buf = protowire.AppendTag(buf, 14, protowire.VarintType)
		buf = protowire.AppendVarint(buf, uint64(c.FileVersion))
	}

	// Field 20: line_ending (string)
	if c.LineEnding != "" {
		buf = protowire.AppendTag(buf, 20, protowire.BytesType)
		buf = protowire.AppendString(buf, c.LineEnding)
	}

	return buf
}

func (c *CursorPosition) marshal() []byte {
	var buf []byte

	// Field 1: line (int32)
	buf = protowire.AppendTag(buf, 1, protowire.VarintType)
	buf = protowire.AppendVarint(buf, uint64(c.Line))

	// Field 2: column (int32)
	buf = protowire.AppendTag(buf, 2, protowire.VarintType)
	buf = protowire.AppendVarint(buf, uint64(c.Column))

	return buf
}

func (c *CppIntentInfo) marshal() []byte {
	var buf []byte

	// Field 1: source (string)
	if c.Source != "" {
		buf = protowire.AppendTag(buf, 1, protowire.BytesType)
		buf = protowire.AppendString(buf, c.Source)
	}

	return buf
}

// StreamCppResponse is the response message for StreamCpp
type StreamCppResponse struct {
	Text       string
	DoneStream bool
	DoneEdit   bool
}

// Unmarshal decodes protobuf bytes into StreamCppResponse
func (r *StreamCppResponse) Unmarshal(data []byte) error {
	for len(data) > 0 {
		fieldNum, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			return protowire.ParseError(n)
		}
		data = data[n:]

		switch fieldNum {
		case 1: // text (string)
			if wireType != protowire.BytesType {
				return protowire.ParseError(-1)
			}
			v, n := protowire.ConsumeString(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			r.Text = v
			data = data[n:]

		case 4: // done_stream (bool)
			if wireType != protowire.VarintType {
				return protowire.ParseError(-1)
			}
			v, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			r.DoneStream = v != 0
			data = data[n:]

		case 13: // done_edit (bool)
			if wireType != protowire.VarintType {
				return protowire.ParseError(-1)
			}
			v, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			r.DoneEdit = v != 0
			data = data[n:]

		default:
			// Skip unknown fields
			n, err := skipField(data, wireType)
			if err != nil {
				return err
			}
			data = data[n:]
		}
	}
	return nil
}

func skipField(data []byte, wireType protowire.Type) (int, error) {
	switch wireType {
	case protowire.VarintType:
		_, n := protowire.ConsumeVarint(data)
		return n, nil
	case protowire.Fixed64Type:
		_, n := protowire.ConsumeFixed64(data)
		return n, nil
	case protowire.BytesType:
		_, n := protowire.ConsumeBytes(data)
		return n, nil
	case protowire.Fixed32Type:
		_, n := protowire.ConsumeFixed32(data)
		return n, nil
	default:
		return 0, protowire.ParseError(-1)
	}
}
