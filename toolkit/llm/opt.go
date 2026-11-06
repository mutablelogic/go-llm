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
	speakModel string
	speakVoice string
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

// WithSpeak sets up the speak tool, which speaks text with the model, by its
// name, such as ollama:phonon:latest, in the voice, unless the model which
// calls it gives another, or the model's own, when it's empty. Without a
// model, there's no speak tool.
func WithSpeak(model, voice string) Opt {
	return func(o *opt) error {
		if model = strings.TrimSpace(model); model == "" {
			return gollm.ErrBadParameter.With("missing model to speak with")
		}
		o.speakModel, o.speakVoice = model, strings.TrimSpace(voice)
		return nil
	}
}
