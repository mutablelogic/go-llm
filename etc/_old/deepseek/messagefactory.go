package deepseek

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type messagefactory struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MESSAGE FACTORY

func (messagefactory) SystemPrompt(prompt string) llm.Completion {
	return &Message{
		RoleContent: RoleContent{
			Role:    "system",
			Content: prompt,
		},
	}
}

func (messagefactory) UserPrompt(prompt string, opts ...llm.Opt) (llm.Completion, error) {
	// Opts not currently used
	return &Message{
		RoleContent: RoleContent{
			Role:    "user",
			Content: prompt,
		},
	}, nil
}

func (messagefactory) ToolResults(results ...llm.ToolResult) ([]llm.Completion, error) {
	// Check for no results
	if len(results) == 0 {
		return nil, llm.ErrBadParameter.Withf("No tool results")
	}

	// Create results
	messages := make([]llm.Completion, 0, len(results))
	for _, result := range results {
		value, err := json.Marshal(result.Value())
		if err != nil {
			return nil, err
		}
		messages = append(messages, &Message{
			RoleContent: RoleContent{
				Role:    "tool",
				Content: string(value),
			},
		})
	}

	// Return success
	return messages, nil
}
