// Package httpclient is a client of the resources' HTTP API
package httpclient

import (
	"context"
	"io"
	"net/http"
	"path"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of the resources' HTTP API, which wraps the base HTTP
// client, with typed methods for its requests
type Client struct {
	*client.Client
}

// upload is the content of an upload, as the request's body, with its
// type, as what it is is in the query
type upload struct {
	io.Reader
	contentType string
}

var _ client.Payload = (*upload)(nil)

// download writes the content of a response, as it's read, and keeps its
// type
type download struct {
	w           io.Writer
	contentType string
}

var _ client.Unmarshaler = (*download)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the resources' API, at the URL, which is the
// server's API endpoint, such as http://localhost:8084/api, with the options
func New(url string, opts ...client.ClientOpt) (*Client, error) {
	c, err := client.New(append(opts, client.OptEndpoint(url))...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListResources returns the resources on their own, or those in the
// request's session, the most recently used first, and how many there are
func (c *Client) ListResources(ctx context.Context, req schema.ResourceListRequest) (*schema.ResourceList, error) {
	var response schema.ResourceList
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("resource"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateResource uploads the content, as it's read, as the request's body,
// as a resource, which is what the request says it is, whose name, when it
// isn't given, is the file's, and whose type, when it isn't given, is
// detected, and returns it. The upload isn't timed out, as a large file can
// take a while.
func (c *Client) CreateResource(ctx context.Context, req schema.ResourceCreateRequest, file string, content io.Reader) (*schema.Resource, error) {
	if content == nil {
		return nil, llm.ErrBadParameter.With("missing content")
	}
	if req.Name == "" && file != "" {
		req.Name = path.Base(file)
	}
	payload := &upload{Reader: content, contentType: req.Type}
	var response schema.Resource
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("resource"), client.OptQuery(req.Query()), client.OptNoTimeout()); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetResource returns the resource with the hash, on its own, or in the
// request's session
func (c *Client) GetResource(ctx context.Context, hash string, req schema.ResourceRequest) (*schema.Resource, error) {
	var response schema.Resource
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath("resource", hash), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// ReadResource writes the content of the resource with the hash, on its
// own, or in the request's session, to the writer, as it's read, and
// returns its type. The download isn't timed out, as a large file can take
// a while.
func (c *Client) ReadResource(ctx context.Context, hash string, req schema.ResourceRequest, w io.Writer) (string, error) {
	if w == nil {
		return "", llm.ErrBadParameter.With("missing writer")
	}
	out := &download{w: w}
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeAny), out, client.OptPath("resource", hash, "content"), client.OptQuery(req.Query()), client.OptNoTimeout()); err != nil {
		return "", err
	}
	return out.contentType, nil
}

// DeleteResource deletes the resource with the hash, on its own, or in the
// request's session, and returns it, as it was
func (c *Client) DeleteResource(ctx context.Context, hash string, req schema.ResourceRequest) (*schema.Resource, error) {
	var response schema.Resource
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodDelete, types.ContentTypeJSON), &response, client.OptPath("resource", hash), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Method returns the method of an upload
func (*upload) Method() string {
	return http.MethodPost
}

// Accept returns the type of the response to an upload
func (*upload) Accept() string {
	return types.ContentTypeJSON
}

// Type returns the type of the content, or application/octet-stream, when
// it isn't known, so it's detected
func (u *upload) Type() string {
	if u.contentType == "" {
		return types.ContentTypeBinary
	}
	return u.contentType
}

// Unmarshal writes the content of the response, and keeps its type
func (d *download) Unmarshal(header http.Header, r io.Reader) error {
	d.contentType = header.Get(types.ContentTypeHeader)
	_, err := io.Copy(d.w, r)
	return err
}
