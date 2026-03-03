package httpclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	types "github.com/mutablelogic/go-server/pkg/types"
)

// errUnauthorized is returned when the server responds with 401.
var errUnauthorized = errors.New("unauthorized")

// IsUnauthorized reports whether err indicates a 401 response.
func IsUnauthorized(err error) bool {
	return errors.Is(err, errUnauthorized)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Connect establishes an MCP session, auto-detecting the transport, and
// immediately starts it running in a background goroutine.
//
// It first tries the 2025-03-26 streamable HTTP transport (POST-first).
// If that fails it retries with the 2024-11-05 SSE transport
// (GET /sse → endpoint event → POST messages)
//
// If the server returns a 401, authFn is called (if non-nil) and the
// connection is retried once. Pass nil to skip the auth retry.
func (c *Client) Connect(ctx context.Context, authFn func(context.Context) error) error {
	if err := c.connect(ctx); err != nil {
		if !IsUnauthorized(err) || authFn == nil {
			return err
		}
		if err := authFn(ctx); err != nil {
			return err
		}
		return c.connect(ctx)
	}
	return nil
}

// Close tears down the background session goroutines and waits for them
// to exit, ensuring no goroutine leaks.
func (c *Client) Close() error {
	if c.runCancel != nil {
		c.runCancel()
	}
	c.runWg.Wait()
	return nil
}

// connect performs the actual transport detection and session startup.
func (c *Client) connect(ctx context.Context) error {
	mc := sdkmcp.NewClient(types.Ptr(c.Implementation), &sdkmcp.ClientOptions{
		KeepAlive: 30 * time.Second,
	})

	// Try the 2025-03-26 streamable HTTP transport first.
	session, err := mc.Connect(ctx, &sdkmcp.StreamableClientTransport{
		Endpoint:   c.url,
		HTTPClient: c.Client.Client,
	}, nil)

	fmt.Println(session)

	return err
}
