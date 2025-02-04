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

// Convenience method to create a session context object with a user prompt
func (model *model) UserPrompt(prompt string, opts ...llm.Opt) llm.Context {
	session := session.NewSession(model, &messagefactory{}, opts...)
	message, err := messagefactory{}.UserPrompt(prompt, opts...)
	if err != nil {
		panic(err)
	}
	session.Append(message)
	return session
}
