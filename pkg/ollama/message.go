package ollama

import (
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Message with text or object content
type Message struct {
	RoleContent
	ToolCallArray `json:"tool_calls,omitempty"`
}

type RoleContent struct {
	Role    string `json:"role,omitempty"`    // assistant, user, tool, system
	Content string `json:"content,omitempty"` // string or array of text, reference, image_url
	Images  []Data `json:"images,omitempty"`  // Image attachments
	ToolResult
}

// A set of tool calls
type ToolCallArray []ToolCall

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
type Data []byte

// ToolResult
type ToolResult struct {
	Name string `json:"name,omitempty"` // function name - when role is tool
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MESSAGE

func (m Message) Num() int {
	return 1
}

func (m Message) Role() string {
	return m.RoleContent.Role
}

func (m Message) Text(index int) string {
	if index != 0 {
		return ""
	}
	return m.Content
}

func (m Message) ToolCalls(index int) []llm.ToolCall {
	if index != 0 {
		return nil
	}

	// Make the tool calls
	calls := make([]llm.ToolCall, 0, len(m.ToolCallArray))
	for _, call := range m.ToolCallArray {
		calls = append(calls, tool.NewCall(fmt.Sprint(call.Function.Index), call.Function.Name, call.Function.Arguments))
	}

	// Return success
	return calls
}
