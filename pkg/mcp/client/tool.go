package client

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// mcpTool wraps an *sdkmcp.Tool received from a server and implements
// llm.Tool, routing Run calls back through the client's CallTool method.
type mcpTool struct {
	t      *sdkmcp.Tool
	client *Client
}

// Ensure mcpTool implements llm.Tool at compile time.
var _ llm.Tool = (*mcpTool)(nil)

///////////////////////////////////////////////////////////////////////////////
// llm.Tool INTERFACE

func (m *mcpTool) Name() string { return m.t.Name }

func (m *mcpTool) Description() string { return m.t.Description }

// InputSchema unmarshals the server-supplied inputSchema (a map[string]any on
// the client side) into a *jsonschema.Schema. Returns nil, nil if not set.
func (m *mcpTool) InputSchema() (*jsonschema.Schema, error) {
	return schemaFromAny(m.t.InputSchema)
}

// OutputSchema unmarshals the optional server-supplied outputSchema. Returns
// nil, nil if the server did not advertise a structured output schema.
func (m *mcpTool) OutputSchema() (*jsonschema.Schema, error) {
	return schemaFromAny(m.t.OutputSchema)
}

// Meta converts MCP ToolAnnotations into llm.ToolMeta.
func (m *mcpTool) Meta() llm.ToolMeta {
	if m.t.Annotations == nil {
		return llm.ToolMeta{}
	}
	return llm.ToolMeta{
		Title:           m.t.Annotations.Title,
		ReadOnlyHint:    m.t.Annotations.ReadOnlyHint,
		DestructiveHint: m.t.Annotations.DestructiveHint,
		IdempotentHint:  m.t.Annotations.IdempotentHint,
		OpenWorldHint:   m.t.Annotations.OpenWorldHint,
	}
}

// Run passes input directly to CallTool and returns the converted plain value.
func (m *mcpTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	return m.client.CallTool(ctx, m.t.Name, input)
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// callToolResult converts *sdkmcp.CallToolResult to (any, error), preserving
// the pkg/llm.Toolkit.Run convention: IsError → Go error, success → plain value.
func callToolResult(res *sdkmcp.CallToolResult) (any, error) {
	if res.IsError {
		if msg := contentText(res.Content); msg != "" {
			return nil, errors.New(msg)
		}
		if res.StructuredContent != nil {
			if data, err := json.Marshal(res.StructuredContent); err == nil {
				return nil, errors.New(string(data))
			}
		}
		if len(res.Content) > 0 {
			if data, err := json.Marshal(res.Content); err == nil {
				return nil, errors.New(string(data))
			}
		}
		return nil, errors.New("tool returned an error")
	}
	if res.StructuredContent != nil {
		return res.StructuredContent, nil
	}
	switch len(res.Content) {
	case 0:
		return nil, nil
	case 1:
		return contentValue(res.Content[0]), nil
	default:
		parts := make([]any, len(res.Content))
		for i, c := range res.Content {
			parts[i] = contentValue(c)
		}
		return parts, nil
	}
}

// contentText joins all text content items into a single string.
func contentText(contents []sdkmcp.Content) string {
	var parts []string
	for _, c := range contents {
		if tc, ok := c.(*sdkmcp.TextContent); ok && tc.Text != "" {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// contentValue returns a plain Go value for a single Content item:
// TextContent → string, everything else → the original typed value.
func contentValue(c sdkmcp.Content) any {
	if tc, ok := c.(*sdkmcp.TextContent); ok {
		return tc.Text
	}
	return c
}

// schemaFromAny re-marshals v (typically map[string]any from JSON) into a
// *jsonschema.Schema. Returns nil, nil if v is nil.
func schemaFromAny(v any) (*jsonschema.Schema, error) {
	if v == nil {
		return nil, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
