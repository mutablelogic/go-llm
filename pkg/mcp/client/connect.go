package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	transport "github.com/mutablelogic/go-client/pkg/transport"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// connectWithAuth establishes an MCP session, auto-detecting the transport.
//
// It first tries the 2025-03-26 streamable HTTP transport (POST-first).
// If that fails it retries with the 2024-11-05 SSE transport
//
// If the server returns a 401, c.authFn is called (if non-nil) with the parsed
// Www-Authenticate fields so it can perform discovery and authorization.
// The connection is then retried once.
func (c *Client) connectWithAuth(ctx context.Context) (*sdkmcp.ClientSession, error) {
	session, err := c.connect(ctx)
	if err != nil {
		if !IsUnauthorized(err) || c.authFn == nil {
			return nil, err
		}
		// Use the resource_metadata URL from the Www-Authenticate header for
		// discovery (RFC 9728); fall back to the server's connect URL.
		// If resource_metadata is relative, resolve it against the connect URL.
		discoveryURL := c.url
		if u := AsUnauthorized(err); u != nil && u.ResourceMetadata() != "" {
			discoveryURL = resolveURL(c.url, u.ResourceMetadata())
		}
		if err := c.authFn(ctx, discoveryURL); err != nil {
			return nil, err
		}
		return c.connect(ctx)
	}
	return session, nil
}

// connect performs the actual transport detection and session startup.
// On success the session is stored on the client.
func (c *Client) connect(ctx context.Context) (*sdkmcp.ClientSession, error) {
	// Shallow-copy the shared *http.Client (which already has the permanent
	// token transport wired in) and wrap with an ephemeral recorder so we can
	// detect 401 responses. The recorder sits outermost. We keep a single
	// *http.Client pointer so both transport attempts share the same recorder.
	client := *c.Client.Client
	recorder := transport.NewRecorder(client.Transport)
	client.Transport = recorder
	httpClient := types.Ptr(client)

	// Try the 2025-03-26 streamable HTTP transport first.
	session, err := c.tryConnect(ctx, recorder, &sdkmcp.StreamableClientTransport{
		Endpoint:   c.url,
		HTTPClient: httpClient,
	})
	if err == nil {
		return session, nil
	}

	// Bail on 401 — let Connect() trigger the auth retry.
	if IsUnauthorized(err) {
		return nil, err
	}

	// Streamable failed for a non-auth reason (e.g. server only speaks the
	// 2024-11-05 SSE protocol). Fall back to SSE transport.
	recorder.Reset()
	return c.tryConnect(ctx, recorder, &sdkmcp.SSEClientTransport{
		Endpoint:   c.url,
		HTTPClient: httpClient,
	})
}

// tryConnect runs a single transport attempt. On success it stores the session
// on c. On 401 it returns an UnauthorizedError joined with the transport error.
func (c *Client) tryConnect(ctx context.Context, recorder *transport.Recorder, t sdkmcp.Transport) (*sdkmcp.ClientSession, error) {
	opts := &sdkmcp.ClientOptions{
		KeepAlive: 30 * time.Second,
	}
	if c.onLoggingMessage != nil {
		fn := c.onLoggingMessage
		opts.LoggingMessageHandler = func(ctx context.Context, req *sdkmcp.LoggingMessageRequest) {
			p := req.Params
			fn(ctx, string(p.Level), p.Logger, p.Data)
		}
	}
	if c.onProgress != nil {
		fn := c.onProgress
		opts.ProgressNotificationHandler = func(ctx context.Context, req *sdkmcp.ProgressNotificationClientRequest) {
			p := req.Params
			fn(ctx, p.ProgressToken, p.Progress, p.Total, p.Message)
		}
	}
	opts.ToolListChangedHandler = func(ctx context.Context, _ *sdkmcp.ToolListChangedRequest) { c.refreshTools(ctx) }
	opts.PromptListChangedHandler = func(ctx context.Context, _ *sdkmcp.PromptListChangedRequest) { c.refreshPrompts(ctx) }
	opts.ResourceListChangedHandler = func(ctx context.Context, _ *sdkmcp.ResourceListChangedRequest) { c.refreshResources(ctx) }
	if c.onResourceUpdated != nil {
		fn := c.onResourceUpdated
		opts.ResourceUpdatedHandler = func(ctx context.Context, req *sdkmcp.ResourceUpdatedNotificationRequest) {
			fn(ctx, c.readResource(ctx, req.Params.URI))
		}
	}
	mc := sdkmcp.NewClient(types.Ptr(c.Implementation), opts)
	session, err := mc.Connect(ctx, t, nil)
	if err != nil && recorder.StatusCode() == http.StatusUnauthorized {
		return nil, errors.Join(NewUnauthorizedError(recorder.Header()), err)
	}
	return session, err
}

// resolveURL resolves ref against base, returning base if either is malformed
// or the resolved result is not an http/https URL.
func resolveURL(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return base
	}
	r, err := url.Parse(ref)
	if err != nil {
		return base
	}
	resolved := b.ResolveReference(r)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return base
	}
	return resolved.String()
}
