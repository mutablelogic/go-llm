package llm

import (
	"encoding/json"
	"strings"
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
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
// which isn't known is nil.
type ModelCap struct {
	Completion    *bool `json:"completion,omitempty" help:"Generates replies to messages"`
	Tools         *bool `json:"tools,omitempty" help:"Calls tools"`
	Thinking      *bool `json:"thinking,omitempty" help:"Thinks, or reasons, before it replies"`
	Vision        *bool `json:"vision,omitempty" help:"Understands images"`
	OCR           *bool `json:"ocr,omitempty" help:"Recognizes the text in images and documents"`
	Transcription *bool `json:"transcription,omitempty" help:"Transcribes speech"`
	Translation   *bool `json:"translation,omitempty" help:"Translates speech into English"`
	Embedding     *bool `json:"embedding,omitempty" help:"Embeds text as vectors"`

	// Temperature is whether the model takes the sampling options, which are
	// the temperature, top_p and top_k, which the registry leaves out, for a
	// model which doesn't, rather than fail, as a session keeps its options
	// as its model changes. It's a setting, rather than what the model does,
	// so it isn't in String.
	Temperature *bool `json:"temperature,omitempty" help:"Takes a sampling temperature, and the other sampling options"`
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
		{"completion", types.Value(c.Completion)},
		{"tools", types.Value(c.Tools)},
		{"thinking", types.Value(c.Thinking)},
		{"vision", types.Value(c.Vision)},
		{"ocr", types.Value(c.OCR)},
		{"transcription", types.Value(c.Transcription)},
		{"translation", types.Value(c.Translation)},
		{"embedding", types.Value(c.Embedding)},
	} {
		if capability.ok {
			result = append(result, capability.name)
		}
	}
	return strings.Join(result, ", ")
}
