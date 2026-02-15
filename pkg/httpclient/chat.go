package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	// Packages
	client "github.com/mutablelogic/go-client"
	gomultipart "github.com/mutablelogic/go-client/pkg/multipart"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ChatOpt is a functional option for the Chat method.
type ChatOpt func(*chatOptions)

type chatOptions struct {
	files    []askFile
	urls     []string
	streamFn opt.StreamFn
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithChatFile adds a file attachment to the chat request.
func WithChatFile(filename string, r io.Reader) ChatOpt {
	return func(o *chatOptions) {
		if r != nil {
			o.files = append(o.files, askFile{filename: filename, body: r})
		}
	}
}

// WithChatURL adds a URL-referenced attachment to the chat request.
func WithChatURL(u string) ChatOpt {
	return func(o *chatOptions) {
		if u != "" {
			o.urls = append(o.urls, u)
		}
	}
}

// WithChatStream enables SSE streaming for the chat request. The callback
// receives streamed text chunks as they arrive (role and text).
func WithChatStream(fn opt.StreamFn) ChatOpt {
	return func(o *chatOptions) {
		o.streamFn = fn
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Chat sends a message within a session with zero or more attachments.
// Use WithChatFile to attach file uploads and WithChatURL to attach URL references.
// A single file with no other attachments uses streaming multipart/form-data;
// all other cases use JSON with base64-encoded file data.
func (c *Client) Chat(ctx context.Context, req schema.ChatRequest, opts ...ChatOpt) (*schema.ChatResponse, error) {
	if req.Session == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}
	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Collect options
	var o chatOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Single file, no URLs → streaming multipart (only for non-SSE)
	if o.streamFn == nil && len(o.files) == 1 && len(o.urls) == 0 && len(req.Attachments) == 0 {
		return c.chatMultipart(ctx, req, o.files[0])
	}

	// Build attachments for any remaining files/URLs
	if err := collectChatAttachments(&req, &o); err != nil {
		return nil, err
	}

	// Streaming SSE path
	if o.streamFn != nil {
		return c.chatStreamSSE(ctx, req, o.streamFn)
	}

	// Default JSON path
	return c.chatJSON(ctx, req)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// chatMultipart sends the request via streaming multipart/form-data with
// a single file attachment.
func (c *Client) chatMultipart(ctx context.Context, req schema.ChatRequest, f askFile) (*schema.ChatResponse, error) {
	httpReq := schema.MultipartChatRequest{
		ChatRequest: req,
		File: gomultipart.File{
			Path: f.filename,
			Body: f.body,
		},
	}

	payload, err := client.NewStreamingMultipartRequest(httpReq, client.ContentTypeJson)
	if err != nil {
		return nil, err
	}

	var response schema.ChatResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("chat")); err != nil {
		return nil, err
	}
	return &response, nil
}

// chatJSON sends the request as JSON with base64-encoded attachments.
func (c *Client) chatJSON(ctx context.Context, req schema.ChatRequest) (*schema.ChatResponse, error) {
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.ChatResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("chat")); err != nil {
		return nil, err
	}
	return &response, nil
}

// chatStreamSSE sends the request as JSON and reads an SSE stream back.
// Delta events are dispatched to the stream callback; the final result
// event is decoded and returned as the ChatResponse.
func (c *Client) chatStreamSSE(ctx context.Context, req schema.ChatRequest, fn opt.StreamFn) (*schema.ChatResponse, error) {
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response *schema.ChatResponse
	var streamErr error

	callback := func(evt client.TextStreamEvent) error {
		switch evt.Event {
		case schema.EventAssistant, schema.EventThinking, schema.EventTool:
			var delta schema.StreamDelta
			if err := json.Unmarshal([]byte(evt.Data), &delta); err == nil {
				fn(delta.Role, delta.Text)
			}
		case schema.EventError:
			var e schema.StreamError
			if err := json.Unmarshal([]byte(evt.Data), &e); err == nil {
				streamErr = fmt.Errorf("%s", e.Error)
			}
		case schema.EventResult:
			var resp schema.ChatResponse
			if err := json.Unmarshal([]byte(evt.Data), &resp); err == nil {
				response = &resp
			}
		}
		return nil
	}

	// Pass a non-nil out so the client proceeds to decode the SSE stream
	// (nil causes an early return before reaching the text-stream decoder).
	var discard struct{}
	if err := c.DoWithContext(ctx, payload, &discard,
		client.OptPath("chat"),
		client.OptReqHeader("Accept", "text/event-stream"),
		client.OptTextStreamCallback(callback),
		client.OptNoTimeout(),
	); err != nil {
		return nil, err
	}
	if streamErr != nil {
		return nil, streamErr
	}
	if response == nil {
		return nil, fmt.Errorf("no result event received in stream")
	}
	return response, nil
}

// collectChatAttachments reads file data and parses URLs into req.Attachments.
func collectChatAttachments(req *schema.ChatRequest, o *chatOptions) error {
	for _, f := range o.files {
		data, err := io.ReadAll(f.body)
		if err != nil {
			return fmt.Errorf("reading file %q: %w", f.filename, err)
		}
		req.Attachments = append(req.Attachments, schema.Attachment{
			Type: http.DetectContentType(data),
			Data: data,
			URL:  &url.URL{Scheme: "file", Path: f.filename},
		})
	}
	for _, u := range o.urls {
		parsed, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("parsing URL %q: %w", u, err)
		}
		req.Attachments = append(req.Attachments, schema.Attachment{
			URL: parsed,
		})
	}
	return nil
}
