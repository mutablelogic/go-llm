// Package llm has the llm namespace of tools, which ask the registry's
// models to do what a model which calls them can't, such as speak text,
// whose results, such as audio, are resources, which are kept, as
// resources of the session the tools run in
package llm

import (
	"context"
	"time"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Models is what the tools ask: the models, a model, by its name, to speak
// text, to recognize the text in an image or document, and to transcribe,
// or translate, speech, such as the registry
type Models interface {
	// ListModels returns the models of every provider, or of the providers
	// with the names, of those which can be listed, and an error for those
	// which can't
	ListModels(ctx context.Context, providers ...string) ([]gollm.Model, error)

	// Name returns the name of the model, by which it's given, which is its
	// provider's name, and its own, such as ollama:phonon:latest
	Name(model gollm.Model) string

	// GetModel returns the model with the name, or ErrNotFound
	GetModel(ctx context.Context, name string) (gollm.Model, error)

	// Speak returns the text, spoken by the model, as audio
	Speak(ctx context.Context, model gollm.Model, text string, opts ...gollm.Opt) (gollm.Resource, gollm.Usage, error)

	// Illustrate returns the images the model makes from the prompt, or by
	// changing, or combining, the images
	Illustrate(ctx context.Context, model gollm.Model, prompt string, images []gollm.Resource, opts ...gollm.Opt) ([]gollm.Resource, gollm.Usage, error)

	// Recognize returns the text in the image, or document
	Recognize(ctx context.Context, model gollm.Model, resource gollm.Resource, opts ...gollm.Opt) (gollm.Document, gollm.Usage, error)

	// Transcribe returns the speech in the audio, as text
	Transcribe(ctx context.Context, model gollm.Model, audio gollm.Resource, stream gollm.TranscriberStreamFn, opts ...gollm.Opt) (gollm.Transcript, gollm.Usage, error)

	// Translate returns the speech in the audio, as English text
	Translate(ctx context.Context, model gollm.Model, audio gollm.Resource, stream gollm.TranscriberStreamFn, opts ...gollm.Opt) (gollm.Transcript, gollm.Usage, error)
}

// ModelsFunc returns the models, or nil, when there are none yet, such as
// when the registry, which has the toolkit, isn't made yet
type ModelsFunc func() Models

// Namespace is the llm namespace, of its tools: models, speak, illustrate,
// ocr, and transcribe,
// each with the model the model which calls it gives, or its default
type Namespace struct {
	tools []gollm.Tool
}

var _ toolkit.Namespace = (*Namespace)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace
const Name = "llm"

// The most time each tool has, which is longer than a tool has by default,
// as asking a model to transcribe an hour of audio, read a long document,
// make an image, or speak a long passage, takes longer
const (
	transcribeTimeout = 5 * time.Minute
	ocrTimeout        = 5 * time.Minute
	illustrateTimeout = 2 * time.Minute
	speakTimeout      = 2 * time.Minute
	resourceTimeout   = time.Minute
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace, whose tools ask the models the function
// returns, with the model which calls them gives, or the defaults the
// options set
func New(models ModelsFunc, opts ...Opt) (*Namespace, error) {
	var o opt
	if models == nil {
		return nil, gollm.ErrBadParameter.With("missing models")
	} else if err := o.apply(opts); err != nil {
		return nil, err
	}
	tools := []gollm.Tool{
		&speak{models: models, model: o.speakModel, voice: o.speakVoice},
		&ocr{models: models, model: o.ocrModel},
		&transcribe{models: models, model: o.transcribeModel},
		&listModels{models: models},
		&illustrate{models: models},
	}
	if o.resources != nil {
		tools = append(tools, &resourceTool{resources: o.resources})
	}
	return &Namespace{tools: tools}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the namespace's name
func (*Namespace) Name() string {
	return Name
}

// Description returns what the namespace's tools are for
func (*Namespace) Description() string {
	return "Models which do what the model which calls them can't: speak text, make and change images, read the text in images and documents, and transcribe speech, and the models which can, by what they can do, and the chat's files, which are read as Markdown"
}

// SystemPrompt returns nothing, as each tool says what it does
func (*Namespace) SystemPrompt() string {
	return ""
}

// Tools returns the tools which are set up
func (n *Namespace) Tools() []gollm.Tool {
	return n.tools
}

// Notify does nothing, as the tools don't change
func (*Namespace) Notify(func(eventschema.EventAction, string)) {}
