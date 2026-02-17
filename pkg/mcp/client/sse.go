package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	mcp "github.com/mutablelogic/go-llm/pkg/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sseTransport holds state for the legacy MCP SSE transport mode, where
// the client maintains a long-lived GET SSE stream and POSTs messages
// to a message endpoint provided by the server.
type sseTransport struct {
	messageURL string                       // endpoint URL for POSTing messages
	pending    map[int64]chan *mcp.Response // request ID → response channel
	mu         sync.Mutex                   // protects pending
	body       io.ReadCloser                // SSE stream body
	cancel     context.CancelFunc           // cancels SSE reader goroutine
	wg         sync.WaitGroup               // waits for SSE reader goroutine
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// connectSSE establishes a long-lived GET SSE connection to the server,
// waits for the endpoint event, and starts the background SSE reader.
// Must be called with c.mu held.
func (c *Client) connectSSE(ctx context.Context) error {
	// Use a background context for the SSE stream so it outlives the init context
	sseCtx, cancel := context.WithCancel(context.Background())

	req, err := http.NewRequestWithContext(sseCtx, http.MethodGet, c.url, nil)
	if err != nil {
		cancel()
		return err
	}
	req.Header.Set("Accept", client.ContentTypeTextStream)
	if c.token.Scheme != "" && c.token.Value != "" {
		req.Header.Set("Authorization", c.token.String())
	}

	resp, err := c.Client.Client.Do(req)
	if err != nil {
		cancel()
		return err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("SSE transport: %s", resp.Status)
	}

	c.sse = &sseTransport{
		pending: make(map[int64]chan *mcp.Response),
		body:    resp.Body,
		cancel:  cancel,
	}

	endpointCh := make(chan string, 1)
	c.sse.wg.Add(1)
	go c.sseReader(sseCtx, resp.Body, endpointCh)

	// Wait for the endpoint event, timeout, or caller cancellation
	select {
	case ep := <-endpointCh:
		base, err := url.Parse(c.url)
		if err != nil {
			cancel()
			return fmt.Errorf("SSE transport: %w", err)
		}
		ref, err := url.Parse(ep)
		if err != nil {
			cancel()
			return fmt.Errorf("SSE transport: invalid endpoint %q: %w", ep, err)
		}
		c.sse.messageURL = base.ResolveReference(ref).String()
		return nil
	case <-time.After(30 * time.Second):
		cancel()
		c.sse = nil
		return fmt.Errorf("SSE transport: timeout waiting for endpoint event")
	case <-ctx.Done():
		cancel()
		c.sse = nil
		return ctx.Err()
	}
}

// initSSE performs the MCP initialize handshake over the SSE transport.
// Must be called with c.mu held. The initReq should be the initialize
// request that was prepared for the Streamable HTTP attempt.
func (c *Client) initSSE(ctx context.Context, initReq mcp.Request) error {
	// Establish SSE connection and wait for endpoint
	if err := c.connectSSE(ctx); err != nil {
		return err
	}

	// Send initialize via SSE transport
	resp, err := c.doSSERPC(ctx, initReq)
	if err != nil {
		return fmt.Errorf("SSE transport initialize: %w", err)
	}

	// Decode server info from result
	if resp.Result != nil {
		if data, err := json.Marshal(resp.Result); err != nil {
			return err
		} else if err := json.Unmarshal(data, &c.server); err != nil {
			return err
		}
	}

	// Send initialized notification (fire-and-forget POST)
	notifReq := mcp.Request{
		Version: mcp.RPCVersion,
		Method:  mcp.NotificationTypeInitialize,
	}
	if err := c.postSSE(ctx, notifReq); err != nil {
		return err
	}

	c.initialized = true
	return nil
}

// sseReader reads SSE events from the stream, dispatching responses to
// pending request channels and notifications to the notify callback.
func (c *Client) sseReader(ctx context.Context, body io.Reader, endpointCh chan<- string) {
	defer c.sse.wg.Done()

	_ = client.NewTextStream().Decode(body, func(event client.TextStreamEvent) error {
		if ctx.Err() != nil {
			return io.EOF
		}

		switch event.Event {
		case "endpoint":
			select {
			case endpointCh <- event.Data:
			default:
			}
			return nil
		case "message", "":
			// Try to decode as a response (has a numeric ID)
			var resp mcp.Response
			if err := event.Json(&resp); err != nil {
				return nil // skip malformed events
			}
			if id, ok := toInt64(resp.ID); ok {
				c.sse.mu.Lock()
				ch, found := c.sse.pending[id]
				c.sse.mu.Unlock()
				if found {
					select {
					case ch <- &resp:
					default:
					}
				}
				return nil
			}
			// Otherwise treat as a notification (has method, no ID)
			var msg mcp.Request
			if err := event.Json(&msg); err != nil {
				return nil
			}
			if msg.Method != "" {
				c.notifyMu.Lock()
				fn := c.notifyFn
				c.notifyMu.Unlock()
				if fn != nil {
					fn(msg.Method, msg.Payload)
				}
			}
			return nil
		default:
			return nil
		}
	})

	// Stream ended: close all pending channels to unblock waiters
	c.sse.mu.Lock()
	for id, ch := range c.sse.pending {
		close(ch)
		delete(c.sse.pending, id)
	}
	c.sse.mu.Unlock()
}

// doSSERPC sends a JSON-RPC request via the SSE transport and waits for the
// response on the SSE stream, matched by request ID.
func (c *Client) doSSERPC(ctx context.Context, req mcp.Request) (*response, error) {
	id, ok := toInt64(req.ID)
	if !ok {
		return nil, fmt.Errorf("SSE transport: request has no numeric ID")
	}

	// Register pending channel for this request ID
	ch := make(chan *mcp.Response, 1)
	c.sse.mu.Lock()
	c.sse.pending[id] = ch
	c.sse.mu.Unlock()

	defer func() {
		c.sse.mu.Lock()
		delete(c.sse.pending, id)
		c.sse.mu.Unlock()
	}()

	// POST the request to the message endpoint
	if err := c.postSSE(ctx, req); err != nil {
		return nil, err
	}

	// Wait for response from SSE stream
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("SSE transport: stream closed while waiting for response")
		}
		if resp.Err != nil {
			return nil, resp.Err
		}
		return &response{Response: *resp}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// postSSE sends a JSON-RPC message to the SSE message endpoint via POST.
// The server is expected to return 200 or 202 with no meaningful body.
func (c *Client) postSSE(ctx context.Context, req mcp.Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.sse.messageURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token.Scheme != "" && c.token.Value != "" {
		httpReq.Header.Set("Authorization", c.token.String())
	}

	resp, err := c.Client.Client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("SSE transport POST: %s", resp.Status)
	}
	return nil
}

// toInt64 converts a JSON-RPC ID (which may be float64 after JSON decoding,
// int64, or json.Number) to int64.
func toInt64(v any) (int64, bool) {
	switch v := v.(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	default:
		return 0, false
	}
}
