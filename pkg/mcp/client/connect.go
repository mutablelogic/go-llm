package client

import (
	"context"
	"time"

	// Packages
	authclient "github.com/djthorpe/go-auth/pkg/httpclient/auth"
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
// If the server returns a 401, c.authFn is called (if non-nil) with any discovered auth
// server metadata, and the error from the failed attempt is returned for retry by Connect().
func (c *Client) connectWithAuth(ctx context.Context, authfn func(config *authclient.Config) error) (*sdkmcp.ClientSession, error) {
	session, err := c.connect(ctx)
	if err != nil {
		// Only retry if the server returned an auth challenge and we have an
		// auth callback configured to resolve it.
		if authclient.AsAuthError(err) == nil {
			return nil, err
		} else if authfn == nil {
			return nil, err
		}

		// Discover the OAuth metadata
		if config, err := c.DiscoverWithError(ctx, err); err != nil {
			return nil, err
		} else if err := authfn(config); err != nil {
			return nil, err
		}

		// Try and reconnect now that we've hopefully fixed the auth problem.
		//  If it fails again, just return the error.
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
	httpClient := types.Ptr(types.Value(c.Client.Client.Client))
	recorder := transport.NewRecorder(httpClient.Transport)
	httpClient.Transport = recorder

	// Try the 2025-03-26 streamable HTTP transport first.
	session, err := c.tryConnect(ctx, recorder, &sdkmcp.StreamableClientTransport{
		Endpoint:   c.url,
		HTTPClient: httpClient,
	})
	if err == nil {
		return session, nil
	} else if autherr := authclient.IsUnauthorized(recorder); autherr != nil {
		return nil, autherr
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
// on c.
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
	opts.ToolListChangedHandler = func(ctx context.Context, _ *sdkmcp.ToolListChangedRequest) {
		c.refreshTools(ctx)
	}
	opts.PromptListChangedHandler = func(ctx context.Context, _ *sdkmcp.PromptListChangedRequest) {
		c.refreshPrompts(ctx)
	}
	opts.ResourceListChangedHandler = func(ctx context.Context, _ *sdkmcp.ResourceListChangedRequest) {
		c.refreshResources(ctx)
	}
	if c.onResourceUpdated != nil {
		fn := c.onResourceUpdated
		opts.ResourceUpdatedHandler = func(ctx context.Context, req *sdkmcp.ResourceUpdatedNotificationRequest) {
			fn(ctx, c.readResource(ctx, req.Params.URI))
		}
	}
	return sdkmcp.NewClient(types.Ptr(c.Implementation), opts).Connect(ctx, t, nil)
}
