package openai

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type messagefactory struct{}

// Message with text or object content
type Message struct {
	RoleContent
	Media *llm.Attachment `json:"audio,omitempty"`
	Calls ToolCalls       `json:"tool_calls,omitempty"`
	*ToolResults
}

var _ llm.Completion = (*Message)(nil)

type RoleContent struct {
	Role    string `json:"role,omitempty"`    // assistant, user, tool, system
	Content any    `json:"content,omitempty"` // string or array of text, reference, image_url
}

type ToolCalls []toolcall

type ToolResults struct {
	Id string `json:"tool_call_id,omitempty"`
}

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
	content[0] = NewTextContext(prompt)
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
				Content: string(value),
			},
			ToolResults: &ToolResults{
				Id: result.Call().Id(),
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

// Return the completion
func (message *Message) Choice(index int) llm.Completion {
	if index != 0 {
		return nil
	}
	return message
}

// Return the text for the last completion
func (message *Message) Text(index int) string {
	if index != 0 {
		return ""
	}
	// If content is text, return it
	if text, ok := message.Content.(string); ok && text != "" {
		return text
	}
	// If content is audio, and there is a caption, return it
	if audio := message.Audio(0); audio != nil && audio.Caption() != "" {
		return audio.Caption()
	}

	// For other kinds, return empty string for the moment
	return ""
}

// Return the audio
func (message *Message) Audio(index int) *llm.Attachment {
	if index != 0 {
		return nil
	}
	return message.Media
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (message *Message) ToolCalls(index int) []llm.ToolCall {
	if index != 0 {
		return nil
	}
	calls := make([]llm.ToolCall, 0, len(message.Calls))
	for _, call := range message.Calls {
		calls = append(calls, call)
	}
	return calls
}
