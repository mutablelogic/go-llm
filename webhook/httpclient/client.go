// Package httpclient is a client of the webhooks' HTTP API
package httpclient

import (
	"context"
	"io"
	"net/http"
	"slices"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of the webhooks' HTTP API, which wraps the base HTTP
// client, with typed methods for its requests
type Client struct {
	*client.Client
}

// hookRequest is a call of a webhook's URL, of its method, with its body,
// of its content type, which is sent as it is
type hookRequest struct {
	method      string
	contentType string
	body        io.Reader
}

var _ client.Payload = (*hookRequest)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	webhookPath = "webhook"
	callPath    = "call"
	hookPath    = "hook"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the webhooks' API, at the URL, which is the
// server's API endpoint, such as http://localhost:8084/api, with the options
func New(url string, opts ...client.ClientOpt) (*Client, error) {
	c, err := client.New(append(slices.Clone(opts), client.OptEndpoint(url), withHookTransport())...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c}, nil
}

///////////////////////////////////////////////////////////////////////////////
// WEBHOOKS

// ListWebhooks returns the webhooks, newest first, as the request selects,
// and how many there are
func (c *Client) ListWebhooks(ctx context.Context, req schema.WebhookListRequest) (*schema.WebhookList, error) {
	var response schema.WebhookList
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath(webhookPath), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateWebhook creates the webhook, and returns it, with its token, and its
// URL, when the server has a public endpoint, which aren't returned again
func (c *Client) CreateWebhook(ctx context.Context, meta schema.WebhookMeta) (*schema.WebhookToken, error) {
	req, err := client.NewJSONRequestEx(http.MethodPost, meta, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.WebhookToken
	if err := c.DoWithContext(ctx, req, &response, client.OptPath(webhookPath)); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetWebhook returns the webhook with the serial number
func (c *Client) GetWebhook(ctx context.Context, id uint64) (*schema.Webhook, error) {
	var response schema.Webhook
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath(webhookPath, id)); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateWebhook changes the webhook with the serial number, as the request
// says, such as its secret, and returns it
func (c *Client) UpdateWebhook(ctx context.Context, id uint64, change schema.WebhookRequest) (*schema.Webhook, error) {
	req, err := client.NewJSONRequestEx(http.MethodPatch, change, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	var response schema.Webhook
	if err := c.DoWithContext(ctx, req, &response, client.OptPath(webhookPath, id)); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteWebhook deletes the webhook with the serial number, and its calls,
// and returns it, as it was
func (c *Client) DeleteWebhook(ctx context.Context, id uint64) (*schema.Webhook, error) {
	var response schema.Webhook
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodDelete, types.ContentTypeJSON), &response, client.OptPath(webhookPath, id)); err != nil {
		return nil, err
	}
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// CALLS

// ListCalls returns the calls, oldest first, as the request selects, after
// the call with its serial number, and how many there are after it
func (c *Client) ListCalls(ctx context.Context, req schema.CallListRequest) (*schema.CallList, error) {
	var response schema.CallList
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath(webhookPath, callPath), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetCall returns the call with the serial number
func (c *Client) GetCall(ctx context.Context, id uint64) (*schema.Call, error) {
	var response schema.Call
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeJSON), &response, client.OptPath(webhookPath, callPath, id)); err != nil {
		return nil, err
	}
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// HOOKS

// Hook calls the URL of the webhook with the token, as its service does,
// with the method, the body, of its content type, which is sent as it is,
// and the headers, such as a signature, and returns the response: the call
// which was kept, or whether it was a delivery which was sent again. The
// token isn't in the client's log, its traces, nor its errors.
func (c *Client) Hook(ctx context.Context, token, method, contentType string, body io.Reader, header http.Header) (*schema.HookResponse, error) {
	if strings.TrimSpace(token) == "" {
		return nil, llm.ErrBadParameter.With("missing webhook token")
	}
	var response schema.HookResponse
	req := &hookRequest{method: method, contentType: contentType, body: body}
	opts := []client.RequestOpt{client.OptPath(hookPath, tokenPlaceholder)}
	for name, values := range header {
		for _, value := range values {
			opts = append(opts, client.OptReqHeader(name, value))
		}
	}
	if err := c.DoWithContext(withToken(ctx, token), req, &response, opts...); err != nil {
		return nil, err
	}
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PAYLOAD

func (r *hookRequest) Method() string { return r.method }

func (r *hookRequest) Accept() string { return types.ContentTypeJSON }

func (r *hookRequest) Type() string { return r.contentType }

func (r *hookRequest) Read(p []byte) (int, error) {
	if r.body == nil {
		return 0, io.EOF
	}
	return r.body.Read(p)
}
