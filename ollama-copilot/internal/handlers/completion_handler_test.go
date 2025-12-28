package handlers

import "testing"

func TestExtractCompletion(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		result   string
		expected string
	}{
		{
			name: "exact match - multiline with partial last line",
			prompt: `export const CURSOR_BEARER_TOKEN = getEnvVar("CURSOR_BEARER_TOKEN");
export const X_CURSOR_CLIENT_VERSION = getEnvVar("X_CURSOR_CLIENT_VERSION");
export const X_REQUEST_ID = getEnvVar("X_REQUEST_ID");
export const X_SESSION_ID = getEnvVar("X_SESSION_ID");

console.log("Ev", "a", "`,
			result: `export const X_CURSOR_CLIENT_VERSION = getEnvVar("X_CURSOR_CLIENT_VERSION");
export const X_REQUEST_ID = getEnvVar("X_REQUEST_ID");
export const X_SESSION_ID = getEnvVar("X_SESSION_ID");

console.log("Ev", "a", "b", "c");
`,
			expected: `b", "c");
`,
		},
		{
			name: "hello world completion",
			prompt: `export const CURSOR_BEARER_TOKEN = getEnvVar("CURSOR_BEARER_TOKEN");
export const X_CURSOR_CLIENT_VERSION = getEnvVar("X_CURSOR_CLIENT_VERSION");
export const X_REQUEST_ID = getEnvVar("X_REQUEST_ID");
export const X_SESSION_ID = getEnvVar("X_SESSION_ID");
console.log("He`,
			result: `export const CURSOR_BEARER_TOKEN = getEnvVar("CURSOR_BEARER_TOKEN");
export const X_CURSOR_CLIENT_VERSION = getEnvVar("X_CURSOR_CLIENT_VERSION");
export const X_REQUEST_ID = getEnvVar("X_REQUEST_ID");
export const X_SESSION_ID = getEnvVar("X_SESSION_ID");
console.log("Hello World");`,
			expected: `llo World");`,
		},
		{
			name:     "empty result",
			prompt:   "some prompt",
			result:   "",
			expected: "",
		},
		{
			name:     "no match in prompt",
			prompt:   "some prompt",
			result:   "completely different text",
			expected: "completely different text",
		},
		{
			name:     "single line prompt and result",
			prompt:   `console.log("Hel`,
			result:   `console.log("Hello World");`,
			expected: `lo World");`,
		},
		{
			name: "duplicate lines - uses last occurrence",
			prompt: `const a = 1;
const a = 1;
const b = `,
			result: `const a = 1;
const b = 2;`,
			expected: `2;`,
		},
		{
			name: "result starts mid-prompt",
			prompt: `line1
line2
line3
partial`,
			result: `line2
line3
partialCompletion`,
			expected: `Completion`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCompletion(tt.prompt, tt.result)
			if got != tt.expected {
				t.Errorf("extractCompletion() = %q, want %q", got, tt.expected)
			}
		})
	}
}
