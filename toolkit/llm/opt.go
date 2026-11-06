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
// such as ollama:phonon:latest, and the voice, unless the model which
// calls it gives another, or the model's own, when it's empty
func WithSpeak(model, voice string) Opt {
	return func(o *opt) error {
		if model = strings.TrimSpace(model); model == "" {
			return gollm.ErrBadParameter.With("missing model to speak with")
		}
		o.speakModel, o.speakVoice = model, strings.TrimSpace(voice)
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
