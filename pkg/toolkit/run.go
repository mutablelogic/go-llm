package toolkit

import (
	"context"
	"errors"
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

	// Gather pending connectors and set their context and cancel function
	var pending []*connector
	for _, c := range tk.connectors {
		if c.cancel == nil && (c.retryAt.IsZero() || !time.Now().Before(c.retryAt)) {
			connCtx, cancel := context.WithCancel(ctx)
			c.ctx = connCtx
			c.cancel = cancel
			c.err = nil
			pending = append(pending, c)
		}
	}

	tk.mu.Unlock()

	// Run pending connectors in parallel and monitor for unexpected errors.
	for _, c := range pending {
		c.wg.Add(1)
		go func(c *connector) {
			defer c.wg.Done()
			err := c.conn.Run(c.ctx)
			tk.mu.Lock()
			if c.cancel != nil {
				c.cancel()
			}

			// Reset context and cancel so this connector can be restarted if desired;
			c.cancel = nil
			c.ctx = nil

			// Remove from namespace map on disconnect, but only if this
			// connector still owns the entry — a re-added connector may have
			// already claimed the same namespace.
			if c.namespace != "" && tk.namespace[c.namespace] == c {
				delete(tk.namespace, c.namespace)
			}

			// Store unexpected errors (not context cancellation/timeout) for
			// collection by stopAllConnectors; clear on clean exit.
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				tk.logger.ErrorContext(c.ctx, "connector stopped", "error", err.Error(), "retries", c.retryCount)
				if !c.retry(err) {
					// Retry ceiling reached — permanently remove the connector.
					tk.logger.ErrorContext(c.ctx, "connector removed after max retries", "retries", c.retryCount)
					for k, v := range tk.connectors {
						if v == c {
							delete(tk.connectors, k)
							break
						}
					}
				}
			} else {
				// Reset error, backoff on clean exit so the next start is immediate.
				c.reset()
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
