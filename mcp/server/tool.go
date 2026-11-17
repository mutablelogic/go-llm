package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"uuid"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// session is what a tool uses as it runs, from the call: its logger, which
// is the server's, and its progress, which is sent to the client, when it
// asked for it
type session struct {
	ctx    context.Context
	req    *sdkmcp.CallToolRequest
	token  any
	logger *slog.Logger
}

var _ toolkit.Session = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// handler returns the handler of the tool's calls, which checks the input,
// as a toolkit does, runs the tool, within its timeout, with a session, and
// returns its result as content, or the error it returned, as the result's
// error, so a model sees it, rather than as the call's
func (s *Server) handler(tool llm.Tool) sdkmcp.ToolHandler {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest) (result *sdkmcp.CallToolResult, err error) {
		ctx, endSpan := otel.StartSpan(s.tracer, ctx, "llm.mcp.server.CallTool",
			attribute.String("arg.namespace", s.ns.Name()),
			attribute.String("arg.name", tool.Name()),
		)
		defer func() { endSpan(err) }()

		// Check the input, as a toolkit does
		var input json.RawMessage
		var token any
		if req.Params != nil {
			input, token = req.Params.Arguments, req.Params.GetProgressToken()
		}
		input, err = toolkit.CheckInput(tool, input)
		if err != nil {
			return errorResult(err), nil
		}

		// Run the tool, within its timeout, with a session
		if timeout := tool.Timeout(); timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		ctx = toolkit.WithSession(ctx, &session{ctx: ctx, req: req, token: token, logger: s.logger.With("namespace", s.ns.Name(), "tool", tool.Name())})

		// The tool runs in a worker, so a tool which doesn't stop once its
		// context is done, or its time is up, doesn't hold the call, and
		// its result is left out, as is a panic, which is its error
		type outcome struct {
			result *sdkmcp.CallToolResult
			err    error
		}
		done := make(chan outcome, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- outcome{result: errorResult(fmt.Errorf("tool %q panicked: %v", tool.Name(), r))}
				}
			}()
			value, err := tool.Run(ctx, input)
			if err != nil {
				done <- outcome{result: errorResult(err)}
				return
			}
			result, err := resultOf(ctx, tool, value)
			done <- outcome{result: result, err: err}
		}()
		select {
		case o := <-done:
			return o.result, o.err
		case <-ctx.Done():
			return errorResult(fmt.Errorf("tool %q: %w", tool.Name(), ctx.Err())), nil
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// SESSION

// ID returns the nil UUID, as the tool isn't run in a session of a model
func (*session) ID() uuid.UUID { return uuid.Nil() }

// Logger returns the server's logger, with the tool
func (s *session) Logger() *slog.Logger { return s.logger }

// Progress sends the tool's progress to the client, when it asked for it
func (s *session) Progress(progress, total float64, message string) {
	if s.token == nil || s.req.Session == nil {
		return
	}
	_ = s.req.Session.NotifyProgress(s.ctx, &sdkmcp.ProgressNotificationParams{
		ProgressToken: s.token,
		Progress:      progress,
		Total:         total,
		Message:       message,
	})
}

// Usage does nothing, as the client doesn't count a tool's tokens
func (*session) Usage(llm.Usage) {}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sdkToolOf returns the tool as the SDK's: its name, title, description,
// and input schema, which is an object, with no properties, when it has
// none, and output schema, when it has one
func sdkToolOf(tool llm.Tool) (*sdkmcp.Tool, error) {
	input := json.RawMessage(`{"type":"object"}`)
	if schema := tool.InputSchema(); schema != nil {
		data, err := json.Marshal(schema)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("tool %q: input schema: %v", tool.Name(), err)
		}
		input = data
	}
	result := &sdkmcp.Tool{
		Name:        tool.Name(),
		Title:       tool.Title(),
		Description: tool.Description(),
		InputSchema: input,
	}
	if schema := tool.OutputSchema(); schema != nil {
		data, err := json.Marshal(schema)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("tool %q: output schema: %v", tool.Name(), err)
		}
		result.OutputSchema = json.RawMessage(data)
	}
	return result, nil
}

// errorResult returns the error as a tool's result
func errorResult(err error) *sdkmcp.CallToolResult {
	result := new(sdkmcp.CallToolResult)
	result.SetError(err)
	return result
}

// resultOf returns the tool's result as content: a value is JSON text,
// which says it's JSON, as go-llm's client reads it, and structured content,
// when the tool has an output schema, and a resource is an image, audio,
// or an embedded resource, by its type, with toolkit.Result's value first
func resultOf(ctx context.Context, tool llm.Tool, value any) (*sdkmcp.CallToolResult, error) {
	result := &sdkmcp.CallToolResult{Content: []sdkmcp.Content{}}
	var resources []llm.Resource
	switch v := value.(type) {
	case nil:
		return result, nil
	case llm.Resource:
		resources = []llm.Resource{v}
		value = nil
	case []llm.Resource:
		resources, value = v, nil
	case toolkit.Result:
		resources, value = v.Resources, v.Value
	case *toolkit.Result:
		if v == nil {
			return result, nil
		}
		resources, value = v.Resources, v.Value
	}

	// The value, as JSON, of which raw JSON which is empty is none
	if value != nil {
		data, ok := value.(json.RawMessage)
		if !ok {
			var err error
			if data, err = json.Marshal(value); err != nil {
				return nil, llm.ErrInternalServerError.Withf("tool %q: %v", tool.Name(), err)
			}
		} else if len(data) > 0 && !json.Valid(data) {
			return nil, llm.ErrInternalServerError.Withf("tool %q: result isn't valid JSON", tool.Name())
		}
		if len(data) > 0 {
			result.Content = append(result.Content, &sdkmcp.TextContent{Text: string(data), Meta: sdkmcp.Meta{types.ContentTypeHeader: types.ContentTypeJSON}})
			if tool.OutputSchema() != nil {
				result.StructuredContent = json.RawMessage(data)
			}
		}
	}

	// The resources, as their content
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		content, err := contentOf(ctx, resource)
		if err != nil {
			return nil, err
		}
		result.Content = append(result.Content, content)
	}
	return result, nil
}

// contentOf returns the resource as content: an image, audio, or otherwise
// an embedded resource, as text, when it's text, or its data, whose URI is
// the resource's, or, for one which has none, as its content is inline,
// resource:///<sha256> of its content, as MCP's needs one which is absolute
func contentOf(ctx context.Context, resource llm.Resource) (sdkmcp.Content, error) {
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return nil, err
	}
	mimeType := resource.Type()
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return &sdkmcp.ImageContent{Data: data, MIMEType: mimeType}, nil
	case strings.HasPrefix(mimeType, "audio/"):
		return &sdkmcp.AudioContent{Data: data, MIMEType: mimeType}, nil
	}
	uri := resource.URI()
	if u, err := url.Parse(uri); err != nil || !u.IsAbs() {
		sum := sha256.Sum256(data)
		uri = "resource:///" + hex.EncodeToString(sum[:])
	}
	contents := &sdkmcp.ResourceContents{URI: uri, MIMEType: mimeType}
	if strings.HasPrefix(mimeType, "text/") || mimeType == types.ContentTypeJSON {
		contents.Text = string(data)
	} else {
		contents.Blob = data
	}
	return &sdkmcp.EmbeddedResource{Resource: contents}, nil
}
