package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	mcp "github.com/mutablelogic/go-llm/pkg/mcp/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// NotifyFunc is called for server notifications received during SSE streams,
// such as progress updates or log messages.
type NotifyFunc func(method string, params json.RawMessage)

// Client is an MCP client that communicates with a remote MCP server
// over HTTP using JSON-RPC 2.0 messages.
type Client struct {
	*client.Client
	id          atomic.Int64
	mu          sync.Mutex
	initialized bool
	sessionId   string
	url         string // server endpoint URL
	server      mcp.ResponseInitialize
	clientInfo  mcp.ClientInfo
	tools       map[string]*mcp.Tool // cached tools by name
	notifyMu    sync.Mutex           // protects notifyFn
	notifyFn    NotifyFunc           // optional notification callback
	token       client.Token         // auth token for raw HTTP requests
	cancel      context.CancelFunc   // cancels the Streamable HTTP listener goroutine
	wg          sync.WaitGroup       // waits for Streamable HTTP listener goroutine
	sse         *sseTransport        // non-nil when using legacy SSE transport
}

// response wraps a JSON-RPC response and captures the Mcp-Session-Id header.
type response struct {
	mcp.Response
	sessionId *string
}

// Ensure response implements client.Unmarshaler
var _ client.Unmarshaler = (*response)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// MCP Streamable HTTP requires both JSON and SSE in Accept header
	mcpAccept = "application/json, text/event-stream"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new MCP client with the given server URL, client info, and options.
func New(url string, info mcp.ClientInfo, opts ...client.ClientOpt) (*Client, error) {
	c := new(Client)
	c.clientInfo = info
	c.url = url

	// Set endpoint and user agent from client info
	defaults := []client.ClientOpt{
		client.OptEndpoint(url),
		client.OptUserAgent(info.Name + "/" + info.Version),
	}
	if httpClient, err := client.New(append(defaults, opts...)...); err != nil {
		return nil, err
	} else {
		c.Client = httpClient
	}
	return c, nil
}

// Close terminates the MCP session. It stops the background listener
// goroutine and sends a DELETE request to the server to end the session.
// It is a no-op if the client has not been initialized.
func (c *Client) Close() error {
	c.mu.Lock()

	if !c.initialized {
		c.mu.Unlock()
		return nil
	}

	// Cancel the Streamable HTTP listener goroutine
	if c.cancel != nil {
		c.cancel()
	}

	// Cancel the SSE transport reader goroutine
	if c.sse != nil && c.sse.cancel != nil {
		c.sse.cancel()
	}
	c.mu.Unlock()

	// Wait for goroutines to exit
	c.wg.Wait()
	if c.sse != nil {
		c.sse.wg.Wait()
		if c.sse.body != nil {
			c.sse.body.Close()
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Send DELETE with session ID to terminate the session (Streamable HTTP only).
	if c.sse == nil && c.sessionId != "" {
		if err := c.DoWithContext(
			context.Background(),
			client.MethodDelete,
			nil,
			client.OptReqHeader("Mcp-Session-Id", c.sessionId),
		); err != nil {
			return err
		}
	}

	// Reset state
	c.initialized = false
	c.sessionId = ""
	c.server = mcp.ResponseInitialize{}
	c.tools = nil
	c.cancel = nil
	c.sse = nil

	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// OnNotification sets a callback for server notifications received during
// SSE streams (e.g., progress updates, log messages, list changes).
// If the client is already initialized and the server supports notifications,
// the background listener is started automatically (Streamable HTTP only;
// the SSE transport reader already handles notifications).
func (c *Client) OnNotification(fn NotifyFunc) {
	c.notifyMu.Lock()
	c.notifyFn = fn
	c.notifyMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Only start background listener for Streamable HTTP; SSE transport
	// already dispatches notifications via sseReader.
	if c.initialized && c.sse == nil && c.cancel == nil && fn != nil && c.server.HasNotifications() {
		c.startListener()
	}
}

// SetToken stores the authentication token for use in raw HTTP requests
// (e.g., the SSE transport stream). This should match the token configured
// via client.OptReqToken on the underlying HTTP client.
func (c *Client) SetToken(token client.Token) {
	c.token = token
}

// ServerInfo returns the server information from the MCP handshake.
// Returns nil if the client has not yet been initialized.
func (c *Client) ServerInfo() *mcp.ResponseInitialize {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.initialized {
		return nil
	}
	return &c.server
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// init performs the MCP initialize handshake if not already done.
// It tries Streamable HTTP first; if the server returns 404 or 405,
// it falls back to the legacy SSE transport.
func (c *Client) init(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Already initialized
	if c.initialized {
		return nil
	}

	// Build the initialize request
	reqId := c.id.Add(1)
	params, err := json.Marshal(mcp.RequestInitialize{
		ProtocolVersion: mcp.ProtocolVersion,
		ClientInfo:      c.clientInfo,
	})
	if err != nil {
		return err
	}

	initReq := mcp.Request{
		Version: mcp.RPCVersion,
		Method:  mcp.MessageTypeInitialize,
		ID:      reqId,
		Payload: params,
	}

	payload, err := client.NewJSONRequestEx(http.MethodPost, initReq, mcpAccept)
	if err != nil {
		return err
	}

	// Send initialize and capture the session ID from response headers
	var resp response
	resp.sessionId = &c.sessionId
	c.notifyMu.Lock()
	fn := c.notifyFn
	c.notifyMu.Unlock()
	opts := []client.RequestOpt{
		client.OptTextStreamCallback(resp.eventCallback(fn)),
	}
	if err := c.DoWithContext(ctx, payload, &resp, opts...); err != nil {
		// If 404 or 405, fall back to legacy SSE transport
		if isHTTPStatus(err, http.StatusNotFound) || isHTTPStatus(err, http.StatusMethodNotAllowed) {
			return c.initSSE(ctx, initReq)
		}
		return err
	}

	// Check for JSON-RPC error
	if resp.Err != nil {
		return resp.Err
	}

	// Decode the result into server info
	if resp.Result != nil {
		if data, err := json.Marshal(resp.Result); err != nil {
			return err
		} else if err := json.Unmarshal(data, &c.server); err != nil {
			return err
		}
	}

	// Send initialized notification (no ID = notification)
	notifReq := mcp.Request{
		Version: mcp.RPCVersion,
		Method:  mcp.NotificationTypeInitialize,
	}
	notifPayload, err := client.NewJSONRequestEx(http.MethodPost, notifReq, mcpAccept)
	if err != nil {
		return err
	}

	// Build request options: include session ID header if we have one
	var notifOpts []client.RequestOpt
	if c.sessionId != "" {
		notifOpts = append(notifOpts, client.OptReqHeader("Mcp-Session-Id", c.sessionId))
	}

	// Notifications return no content, pass nil for out
	if err := c.DoWithContext(ctx, notifPayload, nil, notifOpts...); err != nil {
		return err
	}

	c.initialized = true

	// Start the background listener for server-initiated notifications
	// (Streamable HTTP only; SSE transport handles notifications via sseReader).
	if fn != nil && c.server.HasNotifications() {
		c.startListener()
	}

	return nil
}

// startListener launches a background goroutine that opens a long-lived GET
// SSE stream to the server. This receives out-of-band notifications such as
// tools/list_changed, resources/list_changed, logging, etc.
// Must be called with c.mu held.
func (c *Client) startListener() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.wg.Add(1)
	go c.listen(ctx)
}

// listen runs the Streamable HTTP SSE listener loop with exponential backoff.
// It reconnects automatically when the stream ends, and exits when the context
// is cancelled. It uses the raw *http.Client to avoid holding go-client's mutex,
// which would block all other DoWithContext calls for the lifetime of the stream.
func (c *Client) listen(ctx context.Context) {
	defer c.wg.Done()

	const (
		minBackoff = 1 * time.Second
		maxBackoff = 30 * time.Second
	)
	backoff := minBackoff

	for {
		// Check context before connecting
		if ctx.Err() != nil {
			return
		}

		// Build GET request with SSE accept header and session ID
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
		if err != nil {
			log.Printf("mcp: listener: %v", err)
			return
		}
		req.Header.Set("Accept", client.ContentTypeTextStream)
		if c.token.Scheme != "" && c.token.Value != "" {
			req.Header.Set("Authorization", c.token.String())
		}
		c.mu.Lock()
		if c.sessionId != "" {
			req.Header.Set("Mcp-Session-Id", c.sessionId)
		}
		c.mu.Unlock()

		// Use the underlying *http.Client directly (concurrency-safe, no mutex)
		resp, err := c.Client.Client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("mcp: listener error: %v (reconnecting in %v)", err, backoff)
		} else {
			// 405 means server doesn't support GET SSE — stop trying
			if resp.StatusCode == http.StatusMethodNotAllowed {
				resp.Body.Close()
				return
			}

			if resp.StatusCode == http.StatusOK {
				// Decode SSE stream, dispatching notifications
				_ = client.NewTextStream().Decode(resp.Body, func(event client.TextStreamEvent) error {
					if ctx.Err() != nil {
						return io.EOF
					}
					if event.Event != "message" && event.Event != "" {
						return nil
					}
					var msg mcp.Request
					if err := event.Json(&msg); err != nil {
						return nil // skip malformed events
					}
					c.notifyMu.Lock()
					fn := c.notifyFn
					c.notifyMu.Unlock()
					if fn != nil && msg.Method != "" {
						fn(msg.Method, msg.Payload)
					}
					return nil
				})
				// Reset backoff on successful connection
				backoff = minBackoff
			}
			resp.Body.Close()
		}

		// If context cancelled after request, exit cleanly
		if ctx.Err() != nil {
			return
		}

		// Backoff before reconnecting
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Exponential backoff, capped
		backoff = min(backoff*2, maxBackoff)
	}
}

// isHTTPStatus checks if an error is an HTTP error with the given status code.
func isHTTPStatus(err error, code int) bool {
	var httpErr httpresponse.Err
	if errors.As(err, &httpErr) && int(httpErr) == code {
		return true
	}
	return false
}

// nextId returns the next JSON-RPC request ID.
func (c *Client) nextId() int64 {
	return c.id.Add(1)
}

// reqOpts returns request options including the session ID header.
func (c *Client) reqOpts(extra ...client.RequestOpt) []client.RequestOpt {
	opts := make([]client.RequestOpt, 0, len(extra)+1)
	if c.sessionId != "" {
		opts = append(opts, client.OptReqHeader("Mcp-Session-Id", c.sessionId))
	}
	return append(opts, extra...)
}

// doRPC sends a JSON-RPC request and returns the response. It routes through
// the SSE transport if active, otherwise uses Streamable HTTP.
func (c *Client) doRPC(ctx context.Context, req mcp.Request) (*response, error) {
	// Use SSE transport if connected
	if c.sse != nil {
		return c.doSSERPC(ctx, req)
	}

	// Streamable HTTP: create payload and POST
	payload, err := client.NewJSONRequestEx(http.MethodPost, req, mcpAccept)
	if err != nil {
		return nil, err
	}

	var resp response
	c.notifyMu.Lock()
	fn := c.notifyFn
	c.notifyMu.Unlock()
	opts := c.reqOpts(
		client.OptNoTimeout(),
		client.OptTextStreamCallback(resp.eventCallback(fn)),
	)
	if err := c.DoWithContext(ctx, payload, &resp, opts...); err != nil {
		return nil, err
	}
	if resp.Err != nil {
		return nil, resp.Err
	}
	return &resp, nil
}

///////////////////////////////////////////////////////////////////////////////
// UNMARSHALER

func (r *response) Unmarshal(header http.Header, body io.Reader) error {
	// Capture session ID from response header
	if id := header.Get("Mcp-Session-Id"); id != "" && r.sessionId != nil {
		*r.sessionId = id
	}

	// Check content type - if SSE, fall through to go-client's native handler
	if ct := header.Get("Content-Type"); ct != "" {
		if mimetype, _, err := mime.ParseMediaType(ct); err == nil && mimetype == client.ContentTypeTextStream {
			return httpresponse.ErrNotImplemented
		}
	}

	// Decode the JSON-RPC response
	return json.NewDecoder(body).Decode(&r.Response)
}

// eventCallback returns a TextStreamCallback that decodes SSE events
// containing JSON-RPC messages into this response. Notifications (messages
// without an ID) are dispatched to the notify callback if set.
func (r *response) eventCallback(notifyFn NotifyFunc) client.TextStreamCallback {
	return func(event client.TextStreamEvent) error {
		// MCP sends JSON-RPC responses as "message" events (or default unnamed events)
		if event.Event != "message" && event.Event != "" {
			return nil
		}

		// Peek at the message to check if it's a notification (no ID)
		var msg mcp.Request
		if err := event.Json(&msg); err != nil {
			return err
		}

		// Notifications have a method but no ID
		if msg.ID == nil && msg.Method != "" {
			if notifyFn != nil {
				notifyFn(msg.Method, msg.Payload)
			}
			return nil // keep streaming
		}

		// It's a response — decode into our response struct
		if err := event.Json(&r.Response); err != nil {
			return err
		}
		return io.EOF // got our response, stop streaming
	}
}
