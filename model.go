package llm

import (
	"encoding/json"
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Represents an LLM model
type Model interface {
	Name() string
	Provider() Provider // Model provider
	Description() string
	Created() time.Time
	Aliases() []string                // Model aliases
	Meta() map[string]json.RawMessage // Provider-specific metadata
	InputTokenLimit() *uint           // Input token limit (optional)
	OutputTokenLimit() *uint          // Output token limit (optional)
	Capabilities() ModelCap           // What the model can do, of what's known
}

// ModelCap is what a model can do, of what its provider knows. A capability
// which isn't known is nil. Thinking is the reasoning efforts the model
// accepts, such as "low" and "high", which is empty when it doesn't think,
// and nil when that, or its efforts, aren't known.
type ModelCap struct {
	Completion    *bool    `json:"completion,omitempty" help:"Generates replies to messages"`
	Tools         *bool    `json:"tools,omitempty" help:"Calls tools"`
	Thinking      []string `json:"thinking,omitzero" help:"Reasoning efforts the model accepts, which are none when it doesn't think, and left out when they aren't known"`
	Vision        *bool    `json:"vision,omitempty" help:"Understands images"`
	OCR           *bool    `json:"ocr,omitempty" help:"Recognizes the text in images and documents"`
	Transcription *bool    `json:"transcription,omitempty" help:"Transcribes speech"`
	Translation   *bool    `json:"translation,omitempty" help:"Translates speech into English"`
	Embedding     *bool    `json:"embedding,omitempty" help:"Embeds text as vectors"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// String returns the names of the capabilities which the model is known to
// have, such as "completion, tools, thinking"
func (c ModelCap) String() string {
	var result []string
	for _, capability := range []struct {
		name string
		ok   bool
	}{
		{"completion", isTrue(c.Completion)},
		{"tools", isTrue(c.Tools)},
		{"thinking", len(c.Thinking) > 0},
		{"vision", isTrue(c.Vision)},
		{"ocr", isTrue(c.OCR)},
		{"transcription", isTrue(c.Transcription)},
		{"translation", isTrue(c.Translation)},
		{"embedding", isTrue(c.Embedding)},
	} {
		if capability.ok {
			result = append(result, capability.name)
		}
	}
	return strings.Join(result, ", ")
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// isTrue returns true when the value is known, and true
func isTrue(value *bool) bool {
	return value != nil && *value
}
