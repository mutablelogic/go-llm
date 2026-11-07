package llm

import (
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of the namespace
type Opt func(*opt) error

type opt struct {
	speakModel      string
	speakVoice      string
	ocrModel        string
	transcribeModel string
	resources       Resources
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opts []Opt) error {
	for _, fn := range opts {
		if err := fn(o); err != nil {
			return err
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithSpeak sets the model the speak tool speaks text with, by its name,
// such as ollama:phonon:latest, and the voice, each unless the model which
// calls it gives another. Without a model, the model which calls it gives
// one, and without a voice, it's the model's own. Returns ErrBadParameter
// when neither is set.
func WithSpeak(model, voice string) Opt {
	return func(o *opt) error {
		model, voice = strings.TrimSpace(model), strings.TrimSpace(voice)
		if model == "" && voice == "" {
			return gollm.ErrBadParameter.With("missing model, or voice, to speak with")
		}
		o.speakModel, o.speakVoice = model, voice
		return nil
	}
}

// WithOCR sets the model the ocr tool recognizes the text in an image, or a
// document, such as a PDF, with, by its name, such as
// mistral:mistral-ocr-latest, unless the model which calls it gives another
func WithOCR(model string) Opt {
	return func(o *opt) error {
		if model = strings.TrimSpace(model); model == "" {
			return gollm.ErrBadParameter.With("missing model to recognize text with")
		}
		o.ocrModel = model
		return nil
	}
}

// WithTranscribe sets the model the transcribe tool transcribes, or
// translates into English, the speech in audio with, by its name, such as
// openai:whisper-1, unless the model which calls it gives another
func WithTranscribe(model string) Opt {
	return func(o *opt) error {
		if model = strings.TrimSpace(model); model == "" {
			return gollm.ErrBadParameter.With("missing model to transcribe with")
		}
		o.transcribeModel = model
		return nil
	}
}

// WithResources adds the resource tool, which lists the chat's files, and
// reads a file, by its URI, as Markdown, a part at a time, from the
// resources, such as a resource.Manager
func WithResources(resources Resources) Opt {
	return func(o *opt) error {
		if resources == nil {
			return gollm.ErrBadParameter.With("missing resources")
		}
		o.resources = resources
		return nil
	}
}
