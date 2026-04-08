package client

import (
	"context"
	"fmt"
	"time"

	// Packages

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Connect establishes an MCP session, populates the tool/prompt/resource
// caches and leaves the session available for subsequent API calls.
func (c *Client) Connect(ctx context.Context) error {
	session, err := c.connectWithAuth(ctx, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	if c.session != nil {
		c.mu.Unlock()
		session.Close()
		return fmt.Errorf("client already connected")
	}
	c.session = session
	c.tools = make([]llm.Tool, 0)
	c.prompts = make([]llm.Prompt, 0)
	c.resources = make([]llm.Resource, 0)
	c.mu.Unlock()

	c.refreshTools(ctx)
	c.refreshPrompts(ctx)
	c.refreshResources(ctx)

	if c.onStateChange != nil {
		c.onStateChange(ctx, stateFromSession(session))
	}

	return nil
}

// Close closes the current MCP session and clears all cached state.
func (c *Client) Close() error {
	c.mu.Lock()
	session := c.session
	c.session = nil
	c.tools = nil
	c.prompts = nil
	c.resources = nil
	c.subscribed = nil
	c.mu.Unlock()

	if session == nil {
		return nil
	}
	session.Close()
	return session.Wait()
}

// Run establishes an MCP session (including OAuth if required) and drives it
// until ctx is cancelled or the server closes the connection. It blocks until
// all in-flight messages have been drained and the underlying transport is
// torn down cleanly.
//
// Run is safe to call concurrently with tool-call methods (CallTool, etc.);
// those methods return ErrNotConnected until the session is established and
// continue to work until Run returns.
//
// Server-sent log messages and progress notifications are written to the
// default slog logger while Run is blocking.
func (c *Client) Run(ctx context.Context) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}
	session, err := c.getSession()
	if err != nil {
		return err
	}
	defer c.Close() //nolint:errcheck

	// Ensure the goroutine below is always unblocked when Run returns,
	// even if the server closes the session before ctx is cancelled.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// When the caller cancels ctx, close the session so Wait() unblocks.
	go func() {
		<-ctx.Done()
		session.Close()
	}()

	// Block until the session is fully torn down (server close or ctx cancel).
	return session.Wait()
}

// stateFromSession builds a ConnectorState from the MCP initialize result.
func stateFromSession(session *sdkmcp.ClientSession) *schema.ConnectorState {
	now := time.Now()
	state := &schema.ConnectorState{ConnectedAt: &now}
	if res := session.InitializeResult(); res != nil {
		if res.ServerInfo != nil {
			if res.ServerInfo.Name != "" {
				state.Name = types.Ptr(res.ServerInfo.Name)
			}
			if res.ServerInfo.Version != "" {
				state.Version = types.Ptr(res.ServerInfo.Version)
			}
		}
	}
	return state
}
