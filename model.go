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
	// Chat, and its features
	Chat     *bool `json:"chat,omitempty" help:"Replies to messages, in a conversation"`
	Tools    *bool `json:"tools,omitempty" help:"Calls tools, as it replies"`
	Thinking *bool `json:"thinking,omitempty" help:"Thinks, or reasons, before it replies"`

	// What a model takes, in a conversation
	ImageInput    *bool `json:"image_input,omitempty" help:"Understands images, in a conversation"`
	AudioInput    *bool `json:"audio_input,omitempty" help:"Hears audio, in a conversation"`
	VideoInput    *bool `json:"video_input,omitempty" help:"Watches video, in a conversation"`
	DocumentInput *bool `json:"document_input,omitempty" help:"Reads documents, such as PDFs, in a conversation, rather than recognizing their text first"`

	// What a model makes, other than text
	ImageOutput  *bool `json:"image_output,omitempty" help:"Makes images"`
	SpeechOutput *bool `json:"speech_output,omitempty" help:"Speaks text, as audio"`

	// What a model converts, with its own operation
	Embedding     *bool `json:"embedding,omitempty" help:"Embeds text, and media, as vectors"`
	OCR           *bool `json:"ocr,omitempty" help:"Recognizes the text in images and documents"`
	Transcription *bool `json:"transcription,omitempty" help:"Transcribes speech"`
	Translation   *bool `json:"translation,omitempty" help:"Translates speech into English"`

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
var Capabilities = []string{"chat", "tools", "thinking", "image_input", "audio_input", "video_input", "document_input", "image_output", "speech_output", "embedding", "ocr", "transcription", "translation"}

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
	case "chat":
		value = c.Chat
	case "tools":
		value = c.Tools
	case "thinking":
		value = c.Thinking
	case "image_input":
		value = c.ImageInput
	case "audio_input":
		value = c.AudioInput
	case "video_input":
		value = c.VideoInput
	case "document_input":
		value = c.DocumentInput
	case "image_output":
		value = c.ImageOutput
	case "speech_output":
		value = c.SpeechOutput
	case "embedding":
		value = c.Embedding
	case "ocr":
		value = c.OCR
	case "transcription":
		value = c.Transcription
	case "translation":
		value = c.Translation
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
