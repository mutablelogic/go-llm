// Package llm has the llm namespace of tools, which ask the registry's
// models to do what a model which calls them can't, such as speak text,
// whose results, such as audio, are resources, which are kept, as
// resources of the session the tools run in
package llm

import (
	"context"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Models is what the tools ask: a model, by its name, and a model which
// speaks, to speak text, such as the registry
type Models interface {
	// GetModel returns the model with the name, or ErrNotFound
	GetModel(ctx context.Context, name string) (gollm.Model, error)

	// Speak returns the text, spoken by the model, as audio
	Speak(ctx context.Context, model gollm.Model, text string, opts ...gollm.Opt) (gollm.Resource, gollm.Usage, error)
}

// ModelsFunc returns the models, or nil, when there are none yet, such as
// when the registry, which has the toolkit, isn't made yet
type ModelsFunc func() Models

// Namespace is the llm namespace, of the tools which are set up, such as
// speak, when there's a model to speak with
type Namespace struct {
	tools []gollm.Tool
}

var _ toolkit.Namespace = (*Namespace)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace
const Name = "llm"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace, with the tools the options set up, which ask
// the models the function returns
func New(models ModelsFunc, opts ...Opt) (*Namespace, error) {
	var o opt
	if models == nil {
		return nil, gollm.ErrBadParameter.With("missing models")
	} else if err := o.apply(opts); err != nil {
		return nil, err
	}
	ns := &Namespace{}
	if o.speakModel != "" {
		ns.tools = append(ns.tools, &speak{models: models, model: o.speakModel, voice: o.speakVoice})
	}
	return ns, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the namespace's name
func (*Namespace) Name() string {
	return Name
}

// Description returns what the namespace's tools are for
func (*Namespace) Description() string {
	return "Models which do what the model which calls them can't, such as speak text"
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
