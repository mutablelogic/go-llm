package httpclient

import "context"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run drives the MCP session until ctx is cancelled or the server closes the
// connection. It blocks until all in-flight messages have been drained and the
// underlying transport is torn down cleanly.
//
// Call Connect before Run. If no session is active, Run returns immediately.
// Run is safe to call concurrently with tool-call methods (CallTool, etc.);
// those methods continue to work until Run returns.
//
// Server-sent log messages and progress notifications are written to the
// default slog logger while Run is blocking.
func (c *Client) Run(ctx context.Context) error {
	// Connect (with auth retry if needed) and store the session on c.
	session, err := c.connectWithAuth(ctx)
	if err != nil {
		return err
	}

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
