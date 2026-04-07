package client

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// mcpTool wraps an *sdkmcp.Tool received from a server and implements
// llm.Tool, routing Run calls back through the client's CallTool method.
type mcpTool struct {
	t            *sdkmcp.Tool
	client       *Client
	inputSchema  *jsonschema.Schema
	outputSchema *jsonschema.Schema
}

// Ensure mcpTool implements llm.Tool at compile time.
var _ llm.Tool = (*mcpTool)(nil)

///////////////////////////////////////////////////////////////////////////////
// llm.Tool INTERFACE

func (m *mcpTool) Name() string { return m.t.Name }

func (m *mcpTool) Description() string { return m.t.Description }

// InputSchema unmarshals the server-supplied inputSchema (a map[string]any on
// the client side) into a *jsonschema.Schema. Malformed schemas are ignored.
func (m *mcpTool) InputSchema() *jsonschema.Schema { return m.inputSchema }

// OutputSchema unmarshals the optional server-supplied outputSchema. Returns
// nil if the server did not advertise a structured output schema.
func (m *mcpTool) OutputSchema() *jsonschema.Schema { return m.outputSchema }

// Meta converts MCP Tool fields into llm.ToolMeta.
// Per spec, display-name precedence is: Tool.title > Tool.annotations.title > Tool.name.
func (m *mcpTool) Meta() llm.ToolMeta {
	meta := llm.ToolMeta{
		Title: m.t.Title,
	}
	if m.t.Annotations != nil {
		if meta.Title == "" {
			meta.Title = m.t.Annotations.Title
		}
		meta.ReadOnlyHint = m.t.Annotations.ReadOnlyHint
		meta.DestructiveHint = m.t.Annotations.DestructiveHint
		meta.IdempotentHint = m.t.Annotations.IdempotentHint
		meta.OpenWorldHint = m.t.Annotations.OpenWorldHint
	}
	return meta
}

// Run passes input directly to CallTool, forwarding any session meta
func (m *mcpTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	if len(input) == 0 && m.inputSchema != nil {
		if err := m.inputSchema.Validate(json.RawMessage(`{}`)); err != nil {
			return nil, schema.ErrBadParameter.Withf("input validation failed: %v", err)
		}
	}

	var metaVals []schema.MetaValue
	if sess := toolkit.SessionFromContext(ctx); sess != nil {
		for k, v := range sess.Meta() {
			metaVals = append(metaVals, schema.Meta(k, v))
		}
	}
	return m.client.CallTool(ctx, m.t.Name, input, metaVals...)
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// callToolResult converts *sdkmcp.CallToolResult to (any, error), preserving
// the pkg/tool.Toolkit.Run convention: IsError → Go error, success → plain value.
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
// TextContent with mimetype=application/json → json.RawMessage,
// plain TextContent → string, everything else → the original typed value.
func contentValue(c sdkmcp.Content) any {
	if tc, ok := c.(*sdkmcp.TextContent); ok {
		if tc.Meta[types.ContentTypeHeader] == types.ContentTypeJSON {
			return json.RawMessage(tc.Text)
		}
		return tc.Text
	}
	return c
}

// schemaFromAny re-marshals v (typically map[string]any from JSON) into a
// resolved *jsonschema.Schema. Malformed schemas are ignored.
func schemaFromAny(v any) *jsonschema.Schema {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s, err := jsonschema.FromJSON(data)
	if err != nil {
		return nil
	}
	return s
}
