package toolkit

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run starts all queued connectors and blocks until ctx is cancelled.
// Any connector added after Run starts is picked up on the next tick.
// On cancellation every running connector is stopped and Run waits for
// all of them to finish before returning.
func (tk *toolkit) Run(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), tk.stopAllConnectors())
		case <-ticker.C:
			tk.startPendingConnectors(ctx)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// startPendingConnectors launches a goroutine for every connector that has not
// been started yet (i.e. cancel == nil).
func (tk *toolkit) startPendingConnectors(ctx context.Context) {
	tk.mu.Lock()
	var pending []*connector
	for _, c := range tk.connectors {
		if c.cancel == nil {
			connCtx, cancel := context.WithCancel(ctx)
			c.ctx = connCtx
			c.cancel = cancel
			c.err = nil
			pending = append(pending, c)
		}
	}
	tk.mu.Unlock()

	for _, c := range pending {
		c.wg.Add(1)
		go func(c *connector) {
			defer c.wg.Done()
			err := c.conn.Run(c.ctx)
			tk.mu.Lock()
			if c.cancel != nil {
				c.cancel()
			}
			c.cancel = nil
			c.ctx = nil
			// Store unexpected errors (not context cancellation/timeout) for
			// collection by stopAllConnectors; clear on clean exit.
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				slog.Error("connector stopped", "error", err)
				c.err = err
			} else {
				c.err = nil
			}
			tk.mu.Unlock()
		}(c)
	}
}

// stopAllConnectors cancels every running connector, waits for each to finish,
// and returns any stored unexpected errors joined together.
func (tk *toolkit) stopAllConnectors() error {
	tk.mu.Lock()
	connectors := make([]*connector, 0, len(tk.connectors))
	for _, c := range tk.connectors {
		if c.cancel != nil {
			c.cancel()
		}
		connectors = append(connectors, c)
	}
	tk.mu.Unlock()

	for _, c := range connectors {
		c.wg.Wait()
	}

	var errs error
	tk.mu.RLock()
	for _, c := range connectors {
		errs = errors.Join(errs, c.err)
	}
	tk.mu.RUnlock()
	return errs
}
