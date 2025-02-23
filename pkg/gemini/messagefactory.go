package gemini

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type messagefactory struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MESSAGE FACTORY

func (messagefactory) SystemPrompt(prompt string) llm.Completion {
	return &Content{
		Role: "system",
		Parts: []Part{
			{
				Text: prompt,
			},
		},
	}
}

func (messagefactory) UserPrompt(prompt string, opts ...llm.Opt) (llm.Completion, error) {
	return &Content{
		Role: "user",
		Parts: []Part{
			{
				Text: prompt,
			},
		},
	}, nil
}

func (messagefactory) ToolResults(results ...llm.ToolResult) ([]llm.Completion, error) {
	return nil, llm.ErrNotImplemented
}
