package client

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	// Packages
	jsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// keepAlive is how often a connected server is pinged, so a session whose
// server has gone is closed, once keepAliveFailures pings in a row get no
// reply. The SDK's own keep-alive isn't used, as it closes a session whose
// server replies to a ping with something it can't decode, such as [],
// rather than {}, which some servers do.
var (
	keepAlive         = 30 * time.Second
	keepAliveFailures = 3
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Connect connects to the server, with the streamable HTTP transport, or
// with SSE, when the server hasn't it, and returns once the session is
// initialized, or at once, when it's connected. The context is the
// connection's, as it's made, rather than the session's, which has its
// own, so it lasts until it's disconnected, or the server closes it, as
// SSE's stream lasts as long as the context it's connected with. Returns
// ErrServiceUnavailable when it can't be connected, with each transport's
// error, or the context's error, when it's cancelled.
func (c *Client) Connect(ctx context.Context) error {
	server, err := c.connect(ctx)
	if server != nil && c.change != nil {
		c.change(server, nil)
	}
	return err
}

// connect connects to the server, and returns it, as it said, when it's
// connected now, rather than already, or the error
func (c *Client) connect(ctx context.Context) (*sdkmcp.Implementation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		return nil, nil
	}

	// Try each transport, in turn, unless the caller gave up
	var errs []error
	for _, transport := range c.transports() {
		session, cancel, err := c.dial(ctx, transport)
		if err == nil {
			c.session, c.err, c.cancel = session, nil, cancel
			go c.wait(session)
			go c.keepAlive(session)
			server := &sdkmcp.Implementation{}
			if result := session.InitializeResult(); result != nil && result.ServerInfo != nil {
				server = result.ServerInfo
			}
			return server, nil
		} else if ctx.Err() != nil {
			// The caller gave up, which says nothing of the server
			return nil, ctx.Err()
		}
		errs = append(errs, err)
	}
	c.err = llm.ErrServiceUnavailable.Withf("%s can't be connected to: %w", c.endpoint, errors.Join(errs...))
	return nil, c.err
}

// Disconnect closes the session, when it's connected, or does nothing when
// it isn't
func (c *Client) Disconnect() error {
	c.mu.Lock()
	session, cancel := c.session, c.cancel
	c.session, c.err, c.cancel = nil, nil, nil
	c.mu.Unlock()
	if session == nil {
		return nil
	}
	err := session.Close()
	cancel()
	if c.change != nil {
		c.change(nil, nil)
	}
	return err
}

// Connected returns true when the client has a session with the server,
// which it hasn't once it's disconnected, or the server closes it
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session != nil
}

// Status returns the server's name, and version, as it said when it was
// connected, or nil when it isn't, and the error of the last connection,
// or of the session, when the server closed it, or nil
func (c *Client) Status() (*sdkmcp.Implementation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, c.err
	}
	if result := c.session.InitializeResult(); result != nil && result.ServerInfo != nil {
		return result.ServerInfo, nil
	}
	return &sdkmcp.Implementation{}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// dial connects to the server with the transport, with a context of the
// session's own, which lasts until it's cancelled, as it's disconnected,
// or it ends, and returns as soon as the context is cancelled, so the
// caller can give up, rather than waiting for the attempt to end, which is
// then closed, once it does. Returns the session, and the function which
// cancels its context.
func (c *Client) dial(ctx context.Context, transport sdkmcp.Transport) (*sdkmcp.ClientSession, context.CancelFunc, error) {
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	var opts *sdkmcp.ClientOptions
	if fn := c.toolsChanged; fn != nil {
		opts = &sdkmcp.ClientOptions{ToolListChangedHandler: func(context.Context, *sdkmcp.ToolListChangedRequest) { fn() }}
	}
	type result struct {
		session *sdkmcp.ClientSession
		err     error
	}
	done := make(chan result, 1)
	go func() {
		session, err := sdkmcp.NewClient(&c.implementation, opts).Connect(sessionCtx, transport, nil)
		done <- result{session, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			cancel()
			return nil, nil, r.err
		}
		return r.session, cancel, nil
	case <-ctx.Done():
		// The caller gave up, so the attempt is ended, and closed, when it
		// was connected after all
		cancel()
		go func() {
			if r := <-done; r.session != nil {
				_ = r.session.Close()
			}
		}()
		return nil, nil, ctx.Err()
	}
}

// keepAlive pings the server, every keepAlive, until the session ends, and
// closes it, once keepAliveFailures pings in a row get no reply, so it's
// connected again. A server which replies, even with an error, or with
// what can't be decoded, is there.
func (c *Client) keepAlive(session *sdkmcp.ClientSession) {
	ended := make(chan struct{})
	go func() {
		_ = session.Wait()
		close(ended)
	}()
	ticker := time.NewTicker(keepAlive)
	defer ticker.Stop()
	for failures := 0; ; {
		select {
		case <-ended:
			return
		case <-ticker.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), keepAlive/2)
		err := session.Ping(ctx, nil)
		cancel()
		if replied(err) {
			failures = 0
		} else if failures++; failures >= keepAliveFailures {
			c.mu.Lock()
			if c.session == session {
				c.err = llm.ErrServiceUnavailable.Withf("%s didn't reply to %d pings: %v", c.endpoint, failures, err)
			}
			c.mu.Unlock()
			_ = session.Close()
			return
		}
	}
}

// replied returns true when the error of a ping is none, or says the server
// replied, with an error, such as one which doesn't know ping, or with what
// can't be decoded, such as [], rather than {}
func replied(err error) bool {
	var rpcErr *jsonrpc.Error
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	return err == nil || errors.As(err, &rpcErr) || errors.As(err, &typeErr) || errors.As(err, &syntaxErr)
}

// wait waits for the session to end, such as when the server closes it, or
// its keep-alive fails, and forgets it, with why it ended, unless it's been
// replaced, or disconnected
func (c *Client) wait(session *sdkmcp.ClientSession) {
	err := session.Wait()
	c.mu.Lock()
	ended := c.session == session
	if ended {
		switch {
		case c.err != nil:
			// Why, as the keep-alive closed it
			err = c.err
		case err == nil:
			err = llm.ErrServiceUnavailable.Withf("%s closed the session", c.endpoint)
		}
		c.cancel()
		c.session, c.err, c.cancel = nil, err, nil
	}
	c.mu.Unlock()
	if ended && c.change != nil {
		c.change(nil, err)
	}
}
