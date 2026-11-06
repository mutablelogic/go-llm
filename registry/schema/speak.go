package schema

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SpeakRequest asks a model to speak the text, as audio, with the options,
// such as its voice
type SpeakRequest struct {
	Model string                     `json:"model" help:"Model which speaks text, by name, or by provider and name" example:"ollama:phonon:latest"`
	Text  string                     `json:"text" help:"Text to speak" example:"Hello, world."`
	Opt   map[string]json.RawMessage `json:"opt,omitzero" help:"Options, by key, such as voice=Freya, audio_format=wav, speed=1.2, and instructions=\"Speak calmly\", which the provider decodes" example:"{\"voice\":\"Freya\"}" optional:""`
}

// SpeakMeta is what's said of the audio, in its response's X-Meta header:
// the model which spoke it, by its canonical name, and the usage
type SpeakMeta struct {
	Model string    `json:"model" help:"Canonical name of the model which spoke the text" example:"ollama:phonon:latest"`
	Usage llm.Usage `json:"usage" help:"How long it took, and the characters, or tokens, used, for providers which report them"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// SpeakBodyLimit is the largest request body, in bytes, when text is spoken
const SpeakBodyLimit = 1 << 20

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Opts returns the options, for the provider to decode
func (r SpeakRequest) Opts() []llm.Opt {
	opts := make([]llm.Opt, 0, len(r.Opt))
	for key, value := range r.Opt {
		opts = append(opts, llm.WithOpt(key, value))
	}
	return opts
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r SpeakRequest) String() string {
	return types.Stringify(r)
}
