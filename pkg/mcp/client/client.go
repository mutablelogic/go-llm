package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a thin wrapper around the official MCP Go SDK client.
// It holds the go-client instance only for the OAuth discovery/token dance;
// all MCP protocol work is handled by the SDK's ClientSession.
type Client struct {
	url             string
	trace           io.Writer    // non-nil when HTTP debug tracing is enabled
	httpClient      *http.Client // always non-nil after New(); transport is layered on
	wwwAuthenticate string       // WWW-Authenticate header from the last 401, if any
	session         *sdkmcp.ClientSession
	runCancel       func()         // cancels the background session goroutines
	runWg           sync.WaitGroup // tracks the two background goroutines
	mu              sync.Mutex     // guards messages
	messages        []LogMessage   // log notifications received from the server
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new Client for the given MCP server URL.
func New(url string, opts ...ClientOpt) (*Client, error) {
	c := &Client{url: url}
	for _, o := range opts {
		o(c)
	}
	c.httpClient = &http.Client{Transport: c.baseTransport()}
	return c, nil
}

///////////////////////////////////////////////////////////////////////////////
// CONNECT / DISCONNECT

// Connect establishes an MCP session, auto-detecting the transport, and
// immediately starts it running in a background goroutine.
//
// It first tries the 2025-03-26 streamable HTTP transport (POST-first).
// If that fails it retries with the 2024-11-05 SSE transport
// (GET /sse → endpoint event → POST messages), which is used by servers
// like Linear.
//
// If the server returns a 401, authFn is called (if non-nil) and the
// connection is retried once. Pass nil to skip the auth retry.
//
// Call Wait to block until the background session exits, or cancel ctx to
// stop it early.
func (c *Client) Connect(ctx context.Context, authFn func(context.Context) error) error {
	if err := c.connect(ctx); err != nil {
		if !IsUnauthorized(err) || authFn == nil {
			return err
		}
		if err := authFn(ctx); err != nil {
			return err
		}
		if err := c.connect(ctx); err != nil {
			return err
		}
	}
	return nil
}

// connect performs the actual transport detection and session startup.
func (c *Client) connect(ctx context.Context) error {
	// Reset the message buffer so Messages() only reflects the current session.
	c.mu.Lock()
	c.messages = nil
	c.mu.Unlock()

	mc := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "mcpclient", Version: "v1.0.0"}, &sdkmcp.ClientOptions{
		// Buffer every log notification the server sends so callers can
		// retrieve them via Messages().
		LoggingMessageHandler: func(_ context.Context, req *sdkmcp.LoggingMessageRequest) {
			c.mu.Lock()
			c.messages = append(c.messages, LogMessage{
				Level:  req.Params.Level,
				Logger: req.Params.Logger,
				Data:   req.Params.Data,
			})
			c.mu.Unlock()
		},
	})

	// Wrap the client's transport with a sniffer so that if the server returns
	// a 401 at any point during negotiation we can reliably detect it, even
	// when the SDK subsequently issues follow-up requests (e.g. DELETE to close
	// a partial session) that return a different status such as 405.
	sniffer := &statusSniffer{base: c.httpClient.Transport}
	hc := &http.Client{Transport: sniffer}

	// Try streamable HTTP first.
	session, err := mc.Connect(ctx, &sdkmcp.StreamableClientTransport{
		Endpoint:   c.url,
		HTTPClient: hc,
	}, nil)
	if err != nil {
		if sniffer.saw401 {
			c.wwwAuthenticate = sniffer.wwwAuthenticate
			return errUnauthorized
		}
		// Fall back to old-style SSE transport.
		session, err = mc.Connect(ctx, &sdkmcp.SSEClientTransport{
			Endpoint:   c.url,
			HTTPClient: hc,
		}, nil)
		if err != nil {
			if sniffer.saw401 {
				c.wwwAuthenticate = sniffer.wwwAuthenticate
				return errUnauthorized
			}
			return err
		}
	}
	c.session = session

	// Derive an internal context so Close() can stop the goroutines
	// independently of the caller's ctx.
	runCtx, runCancel := context.WithCancel(ctx)
	c.runCancel = runCancel

	// done is buffered so the inner goroutine never blocks if the outer
	// goroutine exits early via cancellation.
	done := make(chan error, 1)
	c.runWg.Add(2)
	go func() {
		defer c.runWg.Done()
		done <- session.Wait()
	}()
	go func() {
		defer c.runWg.Done()
		select {
		case <-runCtx.Done():
			_ = session.Close()
		case <-done:
		}
	}()
	return nil
}

// Close stops the background session goroutines and waits for both to exit,
// ensuring no goroutines leak.
func (c *Client) Close() error {
	if c.runCancel != nil {
		c.runCancel()
	}
	c.runWg.Wait()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE HELPERS

// baseTransport returns an http.RoundTripper to use as the base for all SDK
// transport HTTP calls. When tracing is enabled it wraps http.DefaultTransport
// with a logger; otherwise it returns http.DefaultTransport directly.
func (c *Client) baseTransport() http.RoundTripper {
	if c.trace != nil {
		return &debugRoundTripper{w: c.trace, base: http.DefaultTransport}
	}
	return http.DefaultTransport
}

// debugRoundTripper logs each HTTP round-trip to w.
type debugRoundTripper struct {
	w    io.Writer
	base http.RoundTripper
}

func (d *debugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Fprintf(d.w, "→ %s %s\n", req.Method, req.URL)
	resp, err := d.base.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(d.w, "← error: %v\n", err)
		return nil, err
	}
	fmt.Fprintf(d.w, "← %s", resp.Status)
	if v := resp.Header.Get("WWW-Authenticate"); v != "" {
		fmt.Fprintf(d.w, " [WWW-Authenticate: %s]", v)
	}
	fmt.Fprintln(d.w)
	return resp, nil
}

// statusSniffer is a transport wrapper that records whether a 401 Unauthorized
// response was received during a connect attempt, and captures the
// WWW-Authenticate header if present.
type statusSniffer struct {
	base            http.RoundTripper
	saw401          bool
	wwwAuthenticate string
}

func (s *statusSniffer) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := s.base.RoundTrip(req)
	if resp != nil && resp.StatusCode == http.StatusUnauthorized {
		s.saw401 = true
		if v := resp.Header.Get("WWW-Authenticate"); v != "" {
			s.wwwAuthenticate = v
		}
	}
	return resp, err
}

// ServerInfo returns the name, version, and protocol version reported by the
// server during the initialize handshake.
func (c *Client) ServerInfo() (name, version, protocol string) {
	if c.session == nil {
		return "", "", ""
	}
	r := c.session.InitializeResult()
	if r == nil {
		return "", "", ""
	}
	if r.ServerInfo != nil {
		name = r.ServerInfo.Name
		version = r.ServerInfo.Version
	}
	protocol = r.ProtocolVersion
	return
}

///////////////////////////////////////////////////////////////////////////////
// TOOLS

// ListTools returns all tools advertised by the server.
func (c *Client) ListTools(ctx context.Context) ([]*sdkmcp.Tool, error) {
	if c.session == nil {
		return nil, nil
	}
	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool calls the named tool with args and returns the result.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	if c.session == nil {
		return nil, nil
	}
	return c.session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

///////////////////////////////////////////////////////////////////////////////
// ERROR HELPERS

// errUnauthorized is returned by connect when the server responds with 401.
var errUnauthorized = errors.New("unauthorized")

// IsUnauthorized reports whether err from Connect indicates a 401.
func IsUnauthorized(err error) bool {
	return errors.Is(err, errUnauthorized)
}
