package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AskOpt is a functional option for the Ask method.
type AskOpt func(*askOptions)

type askOptions struct {
	files []askFile
	urls  []string
}

type askFile struct {
	filename string
	body     io.ReadCloser
}

// /////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithFile adds a file attachment to the ask request.
// WithFile takes ownership of the reader: it will be closed after the
// request completes. The returned AskOpt is single-use; passing the same
// option to multiple Ask calls will fail on the second call because the
// reader will already be consumed and closed.
func WithFile(filename string, r io.Reader) AskOpt {
	return func(o *askOptions) {
		if r != nil {
			rc, ok := r.(io.ReadCloser)
			if !ok {
				rc = io.NopCloser(r)
			}
			o.files = append(o.files, askFile{filename: filename, body: rc})
		}
	}
}

// WithURL adds a URL-referenced attachment to the request.
func WithURL(u string) AskOpt {
	return func(o *askOptions) {
		if u != "" {
			o.urls = append(o.urls, u)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ask sends a stateless text request with zero or more attachments.
// Use WithFile to attach file uploads and WithURL to attach URL references.
// A single file with no other attachments uses streaming multipart/form-data;
// all other cases use JSON with base64-encoded file data.
func (c *Client) Ask(ctx context.Context, req schema.AskRequest, opts ...AskOpt) (*schema.AskResponse, error) {
	if req.Model == nil || *req.Model == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}
	if req.Text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Collect options
	var o askOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Single file, no URLs → streaming multipart
	if len(o.files) == 1 && len(o.urls) == 0 && len(req.Attachments) == 0 {
		return c.askMultipart(ctx, req, o.files[0])
	}

	// Otherwise, build attachments and send as JSON
	if err := collectAttachments(&req, &o); err != nil {
		return nil, err
	}
	return c.askJSON(ctx, req)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// askMultipart sends the request via streaming multipart/form-data with
// a single file attachment.
func (c *Client) askMultipart(ctx context.Context, req schema.AskRequest, f askFile) (*schema.AskResponse, error) {
	defer f.body.Close()
	httpReq := schema.MultipartAskRequest{
		AskRequest: req,
		File: types.File{
			Path: f.filename,
			Body: f.body,
		},
	}

	payload, err := client.NewStreamingMultipartRequest(httpReq, client.ContentTypeJson)
	if err != nil {
		return nil, err
	}

	var response schema.AskResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("ask")); err != nil {
		return nil, err
	}
	return &response, nil
}

// askJSON sends the request as JSON with base64-encoded attachments.
func (c *Client) askJSON(ctx context.Context, req schema.AskRequest) (*schema.AskResponse, error) {
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.AskResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("ask")); err != nil {
		return nil, err
	}
	return &response, nil
}

// collectAttachments reads file data and parses URLs into req.Attachments.
func collectAttachments(req *schema.AskRequest, o *askOptions) error {
	for _, f := range o.files {
		data, err := io.ReadAll(f.body)
		f.body.Close()
		if err != nil {
			return fmt.Errorf("reading file %q: %w", f.filename, err)
		}
		req.Attachments = append(req.Attachments, schema.Attachment{
			ContentType: http.DetectContentType(data),
			Data:        data,
			URL:         &url.URL{Scheme: "file", Path: f.filename},
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
