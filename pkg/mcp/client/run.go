package client

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

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
	// Connect (with auth retry if needed) and store the session on c.
	session, err := c.connectWithAuth(ctx)
	if err != nil {
		return err
	}

	// Expose the session so tool-call methods can use it.
	// Initialize tools to an empty (non-nil) slice so that ListTools never
	// returns nil while connected; nil is reserved to signal "disconnected".
	c.mu.Lock()
	c.session = session
	c.tools = make([]llm.Tool, 0)
	c.mu.Unlock()

	// Populate the tool/prompt/resource caches now that the session is live.
	c.refreshTools(ctx)
	c.refreshPrompts(ctx)
	c.refreshResources(ctx)

	// Clear the session pointer and caches when Run exits.
	defer func() {
		c.mu.Lock()
		c.session = nil
		c.tools = nil
		c.prompts = nil
		c.resources = nil
		c.subscribed = nil
		c.mu.Unlock()
	}()

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
