package ollama

import (
	"encoding/json"
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type messagefactory struct{}

// Message with text or object content
type Message struct {
	RoleContent
	Images []ImageData `json:"images,omitempty"`
	Calls  ToolCalls   `json:"tool_calls,omitempty"`
	*ToolResults
}

var _ llm.Completion = (*Message)(nil)

type RoleContent struct {
	Role    string `json:"role,omitempty"`    // assistant, user, tool, system
	Content string `json:"content,omitempty"` // string or array of text, reference, image_url
}

type ToolCalls []ToolCall

type ToolResults struct {
	Name string `json:"name,omitempty"` // function name - when role is tool
}

type ToolCall struct {
	Type     string           `json:"type"` // function
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Index     int            `json:"index,omitempty"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Data represents the raw binary data of an image file.
type ImageData []byte

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

	// Append image attachments
	attachments := opt.Attachments()
	images := make([]ImageData, 0, len(attachments))
	for _, attachment := range attachments {
		images = append(images, attachment.Data())
	}

	// Return success
	return &Message{
		RoleContent: RoleContent{
			Role:    "user",
			Content: prompt,
		},
		Images: images,
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
				Name: result.Call().Name(),
			},
		})
	}

	// Return success
	return messages, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MESSAGE

// Return the number of completions
func (m Message) Num() int {
	return 1
}

// Return the current session role
func (m Message) Role() string {
	return m.RoleContent.Role
}

// Return the completion
func (message *Message) Choice(index int) llm.Completion {
	if index != 0 {
		return nil
	}
	return message
}

// Return the  text
func (m Message) Text(index int) string {
	if index != 0 {
		return ""
	}
	return m.Content
}

// Return the audio - not supported on ollama
func (message *Message) Attachment(index int) *llm.Attachment {
	return nil
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (m Message) ToolCalls(index int) []llm.ToolCall {
	if index != 0 {
		return nil
	}

	// Make the tool calls
	calls := make([]llm.ToolCall, 0, len(m.Calls))
	for _, call := range m.Calls {
		calls = append(calls, tool.NewCall(fmt.Sprint(call.Function.Index), call.Function.Name, call.Function.Arguments))
	}

	// Return success
	return calls
}
