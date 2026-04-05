package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddTools registers one or more tool.Tool values on the server, converting
// each to the SDK types automatically. Returns an error if any schema cannot
// be marshalled; tools registered before the error are still active.
func (s *Server) AddTools(tools ...llm.Tool) error {
	for _, t := range tools {
		sdkTool, handler, err := sdkToolFromTool(t, s.tracer)
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
func sdkToolFromTool(t llm.Tool, tracer trace.Tracer) (*sdkmcp.Tool, sdkmcp.ToolHandler, error) {
	// Build input schema — SDK accepts any JSON-marshalable value.
	inputSchema := t.InputSchema()
	var err error
	inputSchemaRaw, err := json.Marshal(inputSchema)
	if err != nil {
		return nil, nil, fmt.Errorf("tool %q: marshal input schema: %w", t.Name(), err)
	}

	// Build optional output schema.
	var outputSchemaRaw json.RawMessage
	if outputSchema := t.OutputSchema(); outputSchema != nil {
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

	handler := sdkmcp.ToolHandler(func(ctx context.Context, req *sdkmcp.CallToolRequest) (res *sdkmcp.CallToolResult, retErr error) {
		// Recover from panics in the tool implementation so a misbehaving tool
		// cannot crash the server session.
		defer func() {
			if r := recover(); r != nil {
				result := &sdkmcp.CallToolResult{}
				result.SetError(fmt.Errorf("tool %q panicked: %v", t.Name(), r))
				res, retErr = result, nil
			}
		}()

		// Inject a per-call Session so t.Run can call SessionFromContext(ctx).
		var progressToken any
		var input json.RawMessage
		var meta map[string]any
		if req.Params != nil {
			progressToken = req.Params.GetProgressToken()
			input = req.Params.Arguments
			if len(req.Params.Meta) > 0 {
				meta = map[string]any(req.Params.Meta)
			}
		}
		ctx = withSession(ctx, req.Session, t.Name(), progressToken, meta, tracer)

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
		case strings.HasPrefix(a.ContentType, "image/"):
			return &sdkmcp.ImageContent{Data: a.Data, MIMEType: a.ContentType, Meta: meta}, nil
		case strings.HasPrefix(a.ContentType, "audio/"):
			return &sdkmcp.AudioContent{Data: a.Data, MIMEType: a.ContentType, Meta: meta}, nil
		default:
			// Non-image, non-audio attachment: use text representation.
			return &sdkmcp.TextContent{Text: a.TextContent(), Meta: meta}, nil
		}
	}
	// Plain strings go through as-is — json.Marshal would add extra quotes.
	if s, ok := v.(string); ok {
		return &sdkmcp.TextContent{Text: s}, nil
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("tool %q: marshal output: %w", toolName, err)
	}
	// Tag JSON objects/arrays with a content-type so the client can decode
	// them back as json.RawMessage rather than a plain string.
	if len(out) > 0 && (out[0] == '{' || out[0] == '[') {
		return &sdkmcp.TextContent{
			Text: string(out),
			Meta: sdkmcp.Meta{types.ContentTypeHeader: types.ContentTypeJSON},
		}, nil
	}
	return &sdkmcp.TextContent{Text: string(out)}, nil
}
