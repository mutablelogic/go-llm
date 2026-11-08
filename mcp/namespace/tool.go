package namespace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// tool is one of an MCP server's tools, which is called with its client
type tool struct {
	toolkit.Base
	t            *sdkmcp.Tool
	client       *client.Client
	inputSchema  *jsonschema.Schema
	outputSchema *jsonschema.Schema
	timeout      time.Duration
}

var _ llm.Tool = (*tool)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newTool returns the server's tool, with its input, and output, schemas,
// which are left out, when they aren't valid, and how long it has, or zero
// for the toolkit's default
func newTool(t *sdkmcp.Tool, c *client.Client, timeout time.Duration) *tool {
	return &tool{t: t, client: c, inputSchema: schemaOf(t.InputSchema), outputSchema: schemaOf(t.OutputSchema), timeout: timeout}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the tool's name, on its server
func (t *tool) Name() string { return t.t.Name }

// Title returns the tool's title, for people, or its annotations' title,
// or nothing, when it has neither
func (t *tool) Title() string {
	if t.t.Title != "" {
		return t.t.Title
	} else if t.t.Annotations != nil {
		return t.t.Annotations.Title
	}
	return ""
}

// Description returns what the tool does, or its title, when it says
// nothing of that, so a model has something
func (t *tool) Description() string {
	if t.t.Description != "" {
		return t.t.Description
	} else if t.t.Title != "" {
		return t.t.Title
	} else if t.t.Annotations != nil {
		return t.t.Annotations.Title
	}
	return ""
}

// InputSchema returns the schema of the tool's input
func (t *tool) InputSchema() *jsonschema.Schema { return t.inputSchema }

// OutputSchema returns the schema of the tool's structured output, or nil
func (t *tool) OutputSchema() *jsonschema.Schema { return t.outputSchema }

// Timeout returns how long the tool has, or zero for the toolkit's default
func (t *tool) Timeout() time.Duration { return t.timeout }

// Run calls the tool on its server, with the input, and returns its
// structured output, or its content, or the error it returned
func (t *tool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	result, err := t.client.CallTool(ctx, t.t.Name, input)
	if err != nil {
		return nil, err
	}
	return resultOf(result)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// resultOf returns the tool's result: the error it returned, with its
// text, its structured content, or its content, which is a value, or a
// list of them, when it has more than one, or nil, when it has none
func resultOf(result *sdkmcp.CallToolResult) (any, error) {
	if result == nil {
		return nil, nil
	}
	if result.IsError {
		if text := textOf(result.Content); text != "" {
			return nil, errors.New(text)
		} else if result.StructuredContent != nil {
			if data, err := json.Marshal(result.StructuredContent); err == nil {
				return nil, errors.New(string(data))
			}
		}
		return nil, errors.New("the tool returned an error")
	}
	if result.StructuredContent != nil {
		return result.StructuredContent, nil
	}
	switch len(result.Content) {
	case 0:
		return nil, nil
	case 1:
		return valueOf(result.Content[0]), nil
	}
	values := make([]any, 0, len(result.Content))
	for _, content := range result.Content {
		values = append(values, valueOf(content))
	}
	return values, nil
}

// textOf returns the text of the content, joined by lines
func textOf(contents []sdkmcp.Content) string {
	var lines []string
	for _, content := range contents {
		if text, ok := content.(*sdkmcp.TextContent); ok && text.Text != "" {
			lines = append(lines, text.Text)
		}
	}
	return strings.Join(lines, "\n")
}

// valueOf returns the content as a value: text, which is JSON, when it says
// it is, or is otherwise a string, or the content, as it is, such as an
// image
func valueOf(content sdkmcp.Content) any {
	if text, ok := content.(*sdkmcp.TextContent); ok {
		if text.Meta[types.ContentTypeHeader] == types.ContentTypeJSON && json.Valid([]byte(text.Text)) {
			return json.RawMessage(text.Text)
		}
		return text.Text
	}
	return content
}

// schemaOf returns the schema, which a server sends as JSON, or nil, when
// there's none, or it isn't valid
func schemaOf(v any) *jsonschema.Schema {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	schema, err := jsonschema.FromJSON(data)
	if err != nil {
		return nil
	}
	return schema
}
