package openai

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	session "github.com/mutablelogic/go-llm/pkg/session"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (model *model) Context(opts ...llm.Opt) llm.Context {
	return session.NewSession(model, &messagefactory{}, opts...)
}
