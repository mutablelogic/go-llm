package llm

import (
	"context"
	"encoding/json"
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
	// LoadModel downloads the model with the given name, and otherwise
	// loads the model if it's already present. The progress function, if not
	// nil, is called with the status of the download as it progresses.
	LoadModel(context.Context, string, DownloadProgressFn) (Model, error)

	// DeleteModel deletes the model from local storage
	DeleteModel(context.Context, Model) error

	// UnloadModel unloads the model from memory, keeping it in local
	// storage, so it's loaded again when it's next used
	UnloadModel(context.Context, Model) error
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

// Measurer is an interface for providers which count the tokens of text, as
// a model would, such as to fit what's sent to it into its context
type Measurer interface {
	// Measure returns how many tokens the text is, for the model
	Measure(context.Context, Model, string) (uint, error)
}

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
	// it was spoken in, with its segments and when each was spoken, when
	// they're asked for, so a part of a recording can be found, and the
	// seconds and tokens used. The audio is a resource whose type is audio,
	// such as audio/mpeg, which the provider must support. The stream
	// function, if not nil, is called with the pieces of the transcript as
	// they're transcribed, or once it's transcribed, by a model which
	// doesn't stream. Options which the provider doesn't support are
	// rejected.
	Transcribe(context.Context, Model, Resource, TranscriberStreamFn, ...Opt) (Transcript, Usage, error)

	// Translate returns the speech in the audio, in any language, as English
	// text, with its segments, as Transcribe does, whose language is "en",
	// and the seconds and tokens used
	Translate(context.Context, Model, Resource, TranscriberStreamFn, ...Opt) (Transcript, Usage, error)
}

// TranscriberStreamFn is called with each piece of a transcript as it's
// transcribed: a piece of its text, or, when segments are asked for, a
// segment, once it's transcribed, with empty text. The pieces of text are
// the transcript's text, even for a model which only streams segments.
type TranscriberStreamFn func(text string, segment *TranscriptSegment)

// Recognizer is an interface for providers which recognize the text in
// images and documents, which is optical character recognition (OCR)
type Recognizer interface {
	// Recognize returns the text in the resource, such as an image or a PDF,
	// as a document, with the text of each page, in Markdown, and its
	// regions, when they're asked for, and the pages and tokens used. The
	// resource's type must be one the provider supports, such as image/png
	// or application/pdf. Options which the provider doesn't support are
	// rejected.
	Recognize(context.Context, Model, Resource, ...Opt) (Document, Usage, error)
}

// Transcript is the speech in audio: its text, the language it was spoken
// in, such as "en", when it's known, and its segments, in order, when
// they're asked for
type Transcript struct {
	Text     string              `json:"text" help:"Text of the speech" example:"Hello, world."`
	Language string              `json:"language,omitempty" help:"Language the speech was spoken in, as an ISO 639-1 code, when it's known" example:"en"`
	Segments []TranscriptSegment `json:"segments,omitempty" help:"Segments of the speech, in order, when they're asked for"`
}

// TranscriptSegment is a part of a transcript, such as a sentence, with when
// it starts and ends in the audio, and who spoke it, when the provider
// distinguishes speakers
type TranscriptSegment struct {
	Start   time.Duration `json:"start" help:"When the segment starts in the audio, in nanoseconds" example:"1600000000"`
	End     time.Duration `json:"end" help:"When the segment ends in the audio, in nanoseconds" example:"3500000000"`
	Text    string        `json:"text" help:"Text of the segment" example:"This is a test."`
	Speaker string        `json:"speaker,omitempty" help:"Who spoke the segment, as the provider labels them, when speakers are asked for" example:"A"`
}

// Document is the text recognized in an image or document, by page, in
// order. An image is one page.
type Document struct {
	Pages []DocumentPage `json:"pages" help:"The pages, in order"`
}

// DocumentPage is the text of a page, in Markdown, and its regions, in
// reading order, when they're asked for
type DocumentPage struct {
	Text    string           `json:"text" help:"Text of the page, in Markdown" example:"# Hello, World"`
	Regions []DocumentRegion `json:"regions,omitempty" help:"Regions of the page, in reading order, when they're asked for"`
}

// DocumentRegion is a part of a page, such as a paragraph, a title or a
// table, with its text, and its box. The type is the model's, such as
// "text" or "table", which differ between models.
type DocumentRegion struct {
	Type string     `json:"type,omitempty" help:"What the region is, as the model labels it, such as text, title or table" example:"text"`
	Text string     `json:"text" help:"Text of the region, in Markdown" example:"Hello, World"`
	Box  [4]float64 `json:"box" help:"Box of the region, as its left, top, right and bottom, each a fraction of the page's width or height, from 0 to 1" example:"[0.02,0.02,0.32,0.07]"`
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

	// StreamLog is a line logged by a tool as it runs, such as to say what
	// it's doing, which isn't part of the reply
	StreamLog StreamType = "log"

	// StreamProgress is a tool's progress as it runs, such as that an agent
	// it runs is replying, which replaces its progress before, rather than
	// being kept, as a line it logs is, and isn't part of the reply. Its
	// text is a Progress, as JSON, which ParseProgress reads.
	StreamProgress StreamType = "progress"
)

// Progress is how a tool is getting on, as it runs, as MCP's progress
// notification is: how far it's got, which increases each time, out of the
// total, when it's known, or zero, and a message, such as "Agent segment
// is replying"
type Progress struct {
	Progress float64 `json:"progress"`
	Total    float64 `json:"total,omitempty"`
	Message  string  `json:"message,omitempty"`
}

// Text returns the progress, as JSON, which is the text of a piece of a
// stream which is progress
func (p Progress) Text() string {
	data, err := json.Marshal(p)
	if err != nil {
		return p.Message
	}
	return string(data)
}

// Percent returns how far the progress has got, as a percentage, and
// whether it's known, which it is when there's a total
func (p Progress) Percent() (float64, bool) {
	if p.Total <= 0 {
		return 0, false
	}
	return min(100, max(0, p.Progress/p.Total*100)), true
}

// ParseProgress returns the progress which the text of a piece of a stream
// is, which is a Progress, as JSON, or a message, when it isn't
func ParseProgress(text string) Progress {
	var p Progress
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return Progress{Message: text}
	}
	return p
}

const (
	Ollama     = "ollama"
	Eliza      = "eliza"
	OpenAI     = "openai"
	GoWhisper  = "gowhisper"
	ElevenLabs = "elevenlabs"
)
