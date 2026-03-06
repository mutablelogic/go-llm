package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddTools registers one or more tool.Tool values on the server, converting
// each to the SDK types automatically. Returns an error if any schema cannot
// be marshalled; tools registered before the error are still active.
func (s *Server) AddTools(tools ...tool.Tool) error {
	for _, t := range tools {
		sdkTool, handler, err := sdkToolFromTool(t)
		if err != nil {
			return err
		}
		s.server.AddTool(sdkTool, handler)
	}
	return nil
}

// RemoveTools removes the named tools from the server. Unknown names are
// silently ignored.
func (s *Server) RemoveTools(names ...string) {
	s.server.RemoveTools(names...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// sdkToolFromTool converts a tool.Tool into an *sdkmcp.Tool and sdkmcp.ToolHandler.
func sdkToolFromTool(t tool.Tool) (*sdkmcp.Tool, sdkmcp.ToolHandler, error) {
	// Build input schema — SDK accepts any JSON-marshalable value.
	inputSchema, err := t.InputSchema()
	if err != nil {
		return nil, nil, fmt.Errorf("tool %q: input schema: %w", t.Name(), err)
	}
	inputSchemaRaw, err := json.Marshal(inputSchema)
	if err != nil {
		return nil, nil, fmt.Errorf("tool %q: marshal input schema: %w", t.Name(), err)
	}

	// Build optional output schema.
	var outputSchemaRaw json.RawMessage
	if outputSchema, err := t.OutputSchema(); err != nil {
		return nil, nil, fmt.Errorf("tool %q: output schema: %w", t.Name(), err)
	} else if outputSchema != nil {
		if outputSchemaRaw, err = json.Marshal(outputSchema); err != nil {
			return nil, nil, fmt.Errorf("tool %q: marshal output schema: %w", t.Name(), err)
		}
	}

	// Map ToolMeta hints to SDK annotations.
	meta := t.Meta()
	annotations := &sdkmcp.ToolAnnotations{
		ReadOnlyHint:    meta.ReadOnlyHint,
		IdempotentHint:  meta.IdempotentHint,
		DestructiveHint: meta.DestructiveHint,
		OpenWorldHint:   meta.OpenWorldHint,
	}

	sdkTool := &sdkmcp.Tool{
		Name:        t.Name(),
		Title:       meta.Title,
		Description: t.Description(),
		InputSchema: json.RawMessage(inputSchemaRaw),
		Annotations: annotations,
	}
	if len(outputSchemaRaw) > 0 {
		sdkTool.OutputSchema = outputSchemaRaw
	}

	handler := sdkmcp.ToolHandler(func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		// Inject a per-call Session so t.Run can call SessionFromContext(ctx).
		ctx = withSession(ctx, req.Session, t.Name(), req.Params.GetProgressToken())

		var input json.RawMessage
		if req.Params != nil {
			input = req.Params.Arguments
		}
		out, err := t.Run(ctx, input)
		if err != nil {
			result := &sdkmcp.CallToolResult{}
			result.SetError(err)
			return result, nil
		}
		content, err := contentFromAny(t.Name(), out)
		if err != nil {
			return nil, err
		}
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{content}}, nil
	})

	return sdkTool, handler, nil
}

// contentFromAny converts a tool's return value to an sdkmcp.Content.
// *schema.Attachment with an image/* MIME type maps to ImageContent;
// audio/* maps to AudioContent. Everything else is JSON-marshalled to TextContent.
func contentFromAny(toolName string, v any) (sdkmcp.Content, error) {
	if a, ok := v.(*schema.Attachment); ok && a != nil {
		var meta sdkmcp.Meta
		if a.URL != nil {
			meta = sdkmcp.Meta{"url": a.URL.String()}
		}
		switch {
		case strings.HasPrefix(a.Type, "image/"):
			return &sdkmcp.ImageContent{Data: a.Data, MIMEType: a.Type, Meta: meta}, nil
		case strings.HasPrefix(a.Type, "audio/"):
			return &sdkmcp.AudioContent{Data: a.Data, MIMEType: a.Type, Meta: meta}, nil
		default:
			// Non-image, non-audio attachment: use text representation.
			return &sdkmcp.TextContent{Text: a.TextContent(), Meta: meta}, nil
		}
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("tool %q: marshal output: %w", toolName, err)
	}
	return &sdkmcp.TextContent{Text: string(out)}, nil
}
