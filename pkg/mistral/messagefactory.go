package mistral

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
	// Get attachments
	opt, err := llm.ApplyPromptOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Get attachments, allocate content
	attachments := opt.Attachments()
	content := make([]*Content, 1, len(attachments)+1)

	// Append the text and the attachments
	content[0] = NewTextContext(Text(prompt))
	for _, attachment := range attachments {
		content = append(content, NewImageData(attachment))
	}

	// Return success
	return &Message{
		RoleContent: RoleContent{
			Role:    "user",
			Content: content,
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
				Name:    result.Call().Name(),
				Content: string(value),
				Id:      result.Call().Id(),
			},
		})
	}

	// Return success
	return messages, nil
}
