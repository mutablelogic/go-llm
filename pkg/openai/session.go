package openai

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *model) Context(...llm.Opt) llm.Context {
	return nil
}

// Convenience method to create a session context object
// with a user prompt
func (m *model) UserPrompt(string, ...llm.Opt) llm.Context {
	return nil
}
