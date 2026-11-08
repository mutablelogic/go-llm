package client

import (
	"context"
	"errors"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// keepAlive is how often a connected server is pinged, so a session whose
// server has gone is closed
const keepAlive = 30 * time.Second

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Connect connects to the server, with the streamable HTTP transport, or
// with SSE, when the server hasn't it, and returns once the session is
// initialized, or at once, when it's connected. The context is the
// connection's, as it's made, rather than the session's, which lasts until
// it's disconnected, or the server closes it. Returns
// ErrServiceUnavailable when it can't be connected, with each transport's
// error, or the context's error, when it's cancelled.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		return nil
	}

	// Try each transport, in turn, unless the caller gave up
	var errs []error
	for _, transport := range c.transports() {
		session, err := sdkmcp.NewClient(&c.implementation, &sdkmcp.ClientOptions{KeepAlive: keepAlive}).Connect(ctx, transport, nil)
		if err == nil {
			c.session, c.err = session, nil
			go c.wait(session)
			return nil
		} else if ctx.Err() != nil {
			// The caller gave up, which says nothing of the server
			return ctx.Err()
		}
		errs = append(errs, err)
	}
	c.err = llm.ErrServiceUnavailable.Withf("%s can't be connected to: %w", c.endpoint, errors.Join(errs...))
	return c.err
}

// Disconnect closes the session, when it's connected, or does nothing when
// it isn't
func (c *Client) Disconnect() error {
	c.mu.Lock()
	session := c.session
	c.session, c.err = nil, nil
	c.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
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

// wait waits for the session to end, such as when the server closes it, or
// its keep-alive fails, and forgets it, with why it ended, unless it's been
// replaced, or disconnected
func (c *Client) wait(session *sdkmcp.ClientSession) {
	err := session.Wait()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == session {
		if err == nil {
			err = llm.ErrServiceUnavailable.Withf("%s closed the session", c.endpoint)
		}
		c.session, c.err = nil, err
	}
}
