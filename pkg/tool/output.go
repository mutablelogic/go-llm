package tool

import (
	"context"
	"encoding/json"
	"fmt"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	// OutputToolName is the well-known name for the structured output tool.
	OutputToolName = "submit_output"

	// OutputToolInstruction is appended to the system prompt when the
	// output tool is active, directing the model to call it with the final answer.
	OutputToolInstruction = "Use available tools to gather information. When ready, only call " + OutputToolName + " with your final answer, do not output any other text."
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// OutputTool wraps a JSON schema as a tool, allowing the model to produce
// structured output by "calling" this tool with the desired data.
// This avoids the conflict in providers like Gemini that don't support
// function calling combined with a response JSON schema.
type OutputTool struct {
	schema *jsonschema.Schema
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewOutputTool creates a tool whose parameter schema is the given JSON schema.
// When the model calls this tool, its arguments ARE the structured output.
func NewOutputTool(s *jsonschema.Schema) *OutputTool {
	return &OutputTool{schema: s}
}

///////////////////////////////////////////////////////////////////////////////
// TOOL INTERFACE

func (t *OutputTool) Name() string {
	return OutputToolName
}

func (t *OutputTool) Description() string {
	return "Submit your final structured output. Call this tool when you have completed your task and are ready to return the result."
}

func (t *OutputTool) Schema() (*jsonschema.Schema, error) {
	return t.schema, nil
}

func (t *OutputTool) Run(_ context.Context, input json.RawMessage) (any, error) {
	// The tool's purpose is to capture the structured output — just return it.
	return input, nil
}

// Validate checks that the given JSON conforms to the output schema.
// Returns nil if the data is valid or no schema was provided.
func (t *OutputTool) Validate(data json.RawMessage) error {
	if t.schema == nil {
		return nil
	}
	resolved, err := t.schema.Resolve(nil)
	if err != nil {
		return fmt.Errorf("resolving output schema: %w", err)
	}
	var instance any
	if err := json.Unmarshal(data, &instance); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := resolved.Validate(instance); err != nil {
		return fmt.Errorf("output validation: %w", err)
	}
	return nil
}
