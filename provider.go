package llm

import (
	"context"
	"net/url"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Provider is the interface that interfaces with LLM providers (Anthropic, etc)
type Provider interface {
	// Return the provider name
	Name() string

	// URL returns the provider's normalized endpoint, or nil if it has none
	URL() *url.URL

	// Self returns the underlying provider implementation for capability checks.
	Self() Provider

	// Ping checks the connectivity of the provider and returns an error if not successful
	Ping(context.Context) error

	// ListModels returns the list of available models
	ListModels(context.Context) ([]Model, error)

	// GetModel returns the model with the given name
	GetModel(context.Context, string) (Model, error)
}

// Downloader is an interface for managing model files, for providers which
// store models locally, such as Ollama
type Downloader interface {
	// DownloadModel downloads the model with the given name, and otherwise
	// loads the model if it's already present. The progress function, if not
	// nil, is called with the status of the download as it progresses.
	DownloadModel(context.Context, string, DownloadProgressFn) (Model, error)

	// DeleteModel deletes the model from local storage
	DeleteModel(context.Context, Model) error
}

// DownloadProgressFn is called with the status of a model download, and how
// complete it is, as a percentage from 0 to 100
type DownloadProgressFn func(status string, percent float64)

// Generator is an interface for providers which generate messages, such as
// chat completions
type Generator interface {
	// WithoutSession returns the model's reply to the messages, and the
	// tokens used. Without a session, the messages include any system prompt
	// and earlier turns of the conversation. The model can call the tools,
	// which may be nil, by their names, and the reply has the calls rather
	// than their results. The stream function, if not nil, is called with
	// the pieces of the reply's thinking and text as they're generated.
	// Options which the provider doesn't support are ignored or rejected.
	WithoutSession(context.Context, Model, []Message, []Tool, GeneratorStreamFn, ...Opt) (Message, Usage, error)
}

// GeneratorStreamFn is called with each piece of a message as it's generated,
// with what the piece is, such as the reply's text or the model's thinking
type GeneratorStreamFn func(StreamType, string)

// Embedder is an interface for providers which embed messages as vectors,
// such as for semantic search
type Embedder interface {
	// Embed returns a vector for each of the messages, in the same order, and
	// the tokens used, which are input tokens. A message is embedded by its
	// text. The options can set what the vectors are for (WithTaskType), the
	// title of the document the messages are from (WithTitle), and the
	// number of dimensions of the vectors (WithDimensions), which the model
	// must support. Options which the provider doesn't support are ignored
	// or rejected.
	Embed(context.Context, Model, []Message, ...Opt) ([]Vector, Usage, error)
}

// Vector is an embedding of a message, whose number of dimensions is the
// model's, or as set by WithDimensions
type Vector []float32

// Transcriber is an interface for providers which transcribe speech in
// audio, and translate it into English. A model which can't do one of them,
// such as one which only transcribes, returns ErrNotImplemented for it.
type Transcriber interface {
	// Transcribe returns the speech in the audio as text, in the language
	// it was spoken in, with its segments and when each was spoken, so a
	// part of a recording can be found, and the tokens used. The audio is a
	// resource whose type is audio, such as audio/mpeg, which the provider
	// must support. Options which the provider doesn't support are ignored
	// or rejected.
	Transcribe(context.Context, Model, Resource, ...Opt) (*Transcript, Usage, error)

	// Translate returns the speech in the audio, in any language, as English
	// text, with its segments, as Transcribe does, whose language is "en",
	// and the tokens used
	Translate(context.Context, Model, Resource, ...Opt) (*Transcript, Usage, error)
}

// Recognizer is an interface for providers which recognize the text in
// images and documents, which is optical character recognition (OCR)
type Recognizer interface {
	// Recognize returns the text in the resource, such as an image or a PDF,
	// as a message, whose text is the document's, such as in Markdown, and
	// the tokens used. The resource's type must be one the provider
	// supports, such as image/png or application/pdf. Options which the
	// provider doesn't support are ignored or rejected.
	Recognize(context.Context, Model, Resource, ...Opt) (Message, Usage, error)
}

// Transcript is the speech in audio: its text, the language it was spoken
// in, such as "en", when it's known, and its segments, in order
type Transcript struct {
	Text     string
	Language string
	Segments []TranscriptSegment
}

// TranscriptSegment is a part of a transcript, such as a sentence, with when
// it starts and ends in the audio, and who spoke it, when the provider
// distinguishes speakers
type TranscriptSegment struct {
	Start, End time.Duration
	Text       string
	Speaker    string
}

// StreamType is what a piece of a message is, as it's generated
type StreamType string

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// StreamText is a piece of the text of the reply
	StreamText StreamType = "text"

	// StreamThinking is a piece of the model's thinking, before or while it
	// replies
	StreamThinking StreamType = "thinking"
)

const (
	Ollama = "ollama"
	Eliza  = "eliza"
	OpenAI = "openai"
)
