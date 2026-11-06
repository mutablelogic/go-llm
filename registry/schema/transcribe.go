package schema

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// TranscribeRequest asks a model for the speech in audio, by its type and
// data, as text, in the language it was spoken in, or translated into
// English, with the options
type TranscribeRequest struct {
	Model string                     `json:"model" help:"Model which transcribes speech, by name, or by provider and name" example:"openai:whisper-1"`
	Type  string                     `json:"type,omitempty" help:"MIME type of the audio, which is needed with its data" example:"audio/mpeg" optional:""`
	Data  []byte                     `json:"data,omitempty" help:"Data of the audio, encoded as base64, or its URL" example:"SUQzBAAAAAAA" optional:""`
	URL   string                     `json:"url,omitempty" help:"URI of a resource, which is resource://<hash>, on its own, whose content is read, rather than the data, whose type is the resource's, unless it's given" example:"resource://9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" optional:""`
	Opt   map[string]json.RawMessage `json:"opt,omitzero" help:"Options, by key, such as language=en, segments=true, and speakers=true, which the provider decodes" example:"{\"segments\":true}" optional:""`
}

// TranscribeForm is the form for transcribing, or translating, the speech
// in audio uploaded as a file, such as from a web browser, which is read as
// a TranscribeRequest. The options are a JSON object, as a form's values
// are strings.
type TranscribeForm struct {
	Model string     `json:"model" help:"Model which transcribes speech, by name, or by provider and name" example:"openai:whisper-1"`
	Opt   string     `json:"opt,omitempty" help:"Options, as a JSON object, such as {\"segments\":true}" example:"{\"segments\":true}" optional:""`
	File  types.File `json:"file,omitzero" help:"Audio to transcribe, uploaded as a file"`
}

// TranscribeResponse is the transcript, with its language, when it's known,
// and its segments, when they're asked for, with the model which
// transcribed it, and the seconds, and tokens, used
type TranscribeResponse struct {
	Model string `json:"model" help:"Canonical name of the model which transcribed the speech" example:"openai:whisper-1"`
	llm.Transcript
	Usage llm.Usage `json:"usage" help:"Seconds of audio, and tokens, used, for providers which report them"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// EventSegment is the event of a stream of a transcript with a segment, once
// it's transcribed, when segments are asked for, as its text events have
// each piece of its text
const EventSegment = "segment"

// TranscribeBodyLimit is the largest request body, in bytes, when speech is
// transcribed, which allows for audio of 32 MiB encoded as base64
const TranscribeBodyLimit = 48 << 20

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Resource returns the audio to transcribe, or an error when it has no
// type, or no data
func (r TranscribeRequest) Resource() (llm.Resource, error) {
	return attachment(r.Type, r.Data, r.URL)
}

// Opts returns the options, for the provider to decode
func (r TranscribeRequest) Opts() []llm.Opt {
	opts := make([]llm.Opt, 0, len(r.Opt))
	for key, value := range r.Opt {
		opts = append(opts, llm.WithOpt(key, value))
	}
	return opts
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// String returns the request without its data, which can be large
func (r TranscribeRequest) String() string {
	r.Data = nil
	return types.Stringify(r)
}

func (r TranscribeResponse) String() string {
	return types.Stringify(r)
}
