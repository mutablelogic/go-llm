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
	Speech        *bool `json:"speech,omitempty" help:"Speaks text, as audio"`
	Image         *bool `json:"image,omitempty" help:"Generates images"`

	// Temperature is whether the model takes the sampling options, which are
	// the temperature, top_p and top_k, which the registry leaves out, for a
	// model which doesn't, rather than fail, as a session keeps its options
	// as its model changes. It's a setting, rather than what the model does,
	// so it isn't in String.
	Temperature *bool `json:"temperature,omitempty" help:"Takes a sampling temperature, and the other sampling options"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// Capabilities are the names of what a model can do, in order, which a list
// of models is filtered by. Temperature is a setting, rather than what a
// model does, so isn't one.
var Capabilities = []string{"completion", "tools", "thinking", "vision", "ocr", "transcription", "translation", "embedding", "speech", "image"}

// String returns the names of the capabilities which the model is known to
// have, such as "completion, tools, thinking"
func (c ModelCap) String() string {
	return strings.Join(c.Names(), ", ")
}

// Names returns the names of the capabilities which the model is known to
// have, in the order of Capabilities
func (c ModelCap) Names() []string {
	var result []string
	for _, name := range Capabilities {
		if ok, _ := c.Has(name); ok {
			result = append(result, name)
		}
	}
	return result
}

// Has returns whether the model is known to have the capability, by its
// name, such as vision, which isn't so when it isn't known. Returns
// ErrBadParameter when the name isn't one of Capabilities.
func (c ModelCap) Has(name string) (bool, error) {
	var value *bool
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "completion":
		value = c.Completion
	case "tools":
		value = c.Tools
	case "thinking":
		value = c.Thinking
	case "vision":
		value = c.Vision
	case "ocr":
		value = c.OCR
	case "transcription":
		value = c.Transcription
	case "translation":
		value = c.Translation
	case "embedding":
		value = c.Embedding
	case "speech":
		value = c.Speech
	case "image":
		value = c.Image
	default:
		return false, ErrBadParameter.Withf("unknown capability %q, which isn't one of %s", name, strings.Join(Capabilities, ", "))
	}
	return types.Value(value), nil
}

// WithCapabilities returns the models which are known to have every one of
// the capabilities, by their names, in their order, or all of them, when
// there are none. Returns ErrBadParameter when a name isn't one of
// Capabilities.
func WithCapabilities(models []Model, capabilities ...string) ([]Model, error) {
	if err := ValidCapabilities(capabilities...); err != nil {
		return nil, err
	}
	if len(capabilities) == 0 {
		return models, nil
	}
	result := make([]Model, 0, len(models))
	for _, model := range models {
		if hasAll(model.Capabilities(), capabilities) {
			result = append(result, model)
		}
	}
	return result, nil
}

// ValidCapabilities returns ErrBadParameter when any of the names isn't one
// of Capabilities, so a request for them is refused before models are
// listed
func ValidCapabilities(capabilities ...string) error {
	for _, name := range capabilities {
		if _, err := (ModelCap{}).Has(name); err != nil {
			return err
		}
	}
	return nil
}

// hasAll returns whether the capabilities include every one of the names,
// which are known to be capabilities
func hasAll(c ModelCap, names []string) bool {
	for _, name := range names {
		if ok, _ := c.Has(name); !ok {
			return false
		}
	}
	return true
}
