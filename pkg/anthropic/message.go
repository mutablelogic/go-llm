package anthropic

import (
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Message with text or object content
type Message struct {
	RoleContent
}

var _ llm.Completion = (*Message)(nil)

type RoleContent struct {
	Role    string     `json:"role"`
	Content []*Content `json:"content,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Message) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MESSAGE

// Return the number of completions
func (Message) Num() int {
	return 1
}

// Return the current session role
func (message Message) Role() string {
	return message.RoleContent.Role
}

// Return the completion
func (message Message) Choice(index int) llm.Completion {
	if index != 0 {
		return nil
	}
	return message
}

// Return the text for the last completion
func (message Message) Text(index int) string {
	if index != 0 {
		return ""
	}
	var text []string
	for _, content := range message.RoleContent.Content {
		if content.Type == "text" {
			text = append(text, content.ContentText.Text)
		}
	}
	return strings.Join(text, "\n")
}

// Return the audio - unsupported
func (Message) Audio(index int) *llm.Attachment {
	return nil
}

func (message Message) ToolCalls(index int) []llm.ToolCall {
	if index != 0 {
		return nil
	}

	// Gather tool calls
	var result []llm.ToolCall
	for _, content := range message.Content {
		if content.Type == "tool_use" {
			result = append(result, tool.NewCall(content.ContentTool.Id, content.ContentTool.Name, content.ContentTool.Input))
		}
	}
	return result
}
