package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// A tool can be called from an LLM
type Tool interface {
	// Return the provider of the tool
	Provider() string

	// Return the name of the tool
	Name() string

	// Return the description of the tool
	Description() string

	// Tool parameters
	Params() []ToolParameter

	// Execute the tool with a specific tool
	Run(context.Context, *ToolCall) (*ToolResult, error)
}

// A tool parameter
type ToolParameter struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// A call to a tool
type ToolCall struct {
	Id   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// The result of a tool call
type ToolResult struct {
	Id     string         `json:"id"`
	Result map[string]any `json:"result,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the arguments for the call as a JSON
func (t *ToolCall) Json() string {
	data, err := json.MarshalIndent(t.Args, "", "  ")
	if err != nil {
		return err.Error()
	} else {
		return string(data)
	}
}

// Return role for the tool result
func (t *ToolResult) Role() string {
	return "tool"
}

// Return parameter as a string
func (t *ToolCall) String(name string) (string, error) {
	v, ok := t.Args[name]
	if !ok {
		return "", ErrNotFound.Withf("%q not found", name)
	}
	return fmt.Sprint(v), nil
}

// Return parameter as an integer
func (t *ToolCall) Int(name string) (int, error) {
	v, ok := t.Args[name]
	if !ok {
		return 0, ErrNotFound.Withf("%q not found", name)
	}
	switch v := v.(type) {
	case int:
		return v, nil
	case string:
		if v_, err := strconv.ParseInt(v, 10, 32); err != nil {
			return 0, ErrBadParameter.Withf("%q: %v", name, err)
		} else {
			return int(v_), nil
		}
	default:
		return 0, ErrBadParameter.Withf("%q: Expected integer, got %T", name, v)
	}
}
