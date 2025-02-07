package anthropic

import (
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
			Content: []*Content{NewTextContent(prompt)},
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
	contents := make([]*Content, 1, len(attachments)+1)

	// Append the text and the attachments
	contents[0] = NewTextContent(prompt)
	for _, attachment := range attachments {
		if content, err := NewAttachment(attachment, optEphemeral(opt), optCitations(opt)); err != nil {
			return nil, err
		} else {
			contents = append(contents, content)
		}
	}

	// Return success
	return &Message{
		RoleContent: RoleContent{
			Role:    "user",
			Content: contents,
		},
	}, nil
}

func (messagefactory) ToolResults(results ...llm.ToolResult) ([]llm.Completion, error) {
	// Check for no results
	if len(results) == 0 {
		return nil, llm.ErrBadParameter.Withf("No tool results")
	}

	// Create user message
	message := Message{
		RoleContent{
			Role:    "user",
			Content: make([]*Content, 0, len(results)),
		},
	}
	for _, result := range results {
		message.RoleContent.Content = append(message.RoleContent.Content, NewToolResultContent(result))
	}

	// Return success
	return []llm.Completion{message}, nil
}
