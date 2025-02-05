package openai

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	session "github.com/mutablelogic/go-llm/pkg/session"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type messagefactory struct{}

// Message with text or object content
type Message struct {
	RoleContent
}

var _ llm.Completion = (*Message)(nil)

type RoleContent struct {
	Role    string `json:"role,omitempty"`    // assistant, user, tool, system
	Content any    `json:"content,omitempty"` // string or array of text, reference, image_url
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MESSAGE FACTORY

func (messagefactory) SystemPrompt(prompt string) session.Message {
	return &Message{
		RoleContent: RoleContent{
			Role:    "system",
			Content: prompt,
		},
	}
}

func (messagefactory) UserPrompt(prompt string, opts ...llm.Opt) (session.Message, error) {
	// TODO: Attachments
	// Return success
	return &Message{
		RoleContent: RoleContent{
			Role:    "user",
			Content: prompt,
		},
	}, nil
}

func (messagefactory) ToolResults(results ...llm.ToolResult) ([]session.Message, error) {
	// Check for no results
	if len(results) == 0 {
		return nil, llm.ErrBadParameter.Withf("No tool results")
	}

	// Create results
	messages := make([]session.Message, 0, len(results))
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

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - COMPLETION

// Return the number of completions
func (message *Message) Num() int {
	return 1
}

// Return the current session role
func (message *Message) Role() string {
	return message.RoleContent.Role
}

// Return the text for the last completion
func (message *Message) Text(index int) string {
	if index != 0 {
		return ""
	}
	// If content is text, return it
	if text, ok := message.Content.(string); ok {
		return text
	}
	// For other kinds, return empty string for the moment
	return ""
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (message *Message) ToolCalls(index int) []llm.ToolCall {
	if index != 0 {
		return nil
	}
	// TODO
	return nil
}
