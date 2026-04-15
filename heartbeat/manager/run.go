package heartbeat

import (
	"context"
	"time"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat/schema"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run starts the maturity-check loop and blocks until ctx is cancelled.
// It returns nil when ctx is done and satisfies the llm.Connector interface.
func (m *Manager) Run(ctx context.Context) error {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

type markFiredWriter struct {
	fired bool
}

func (markFiredWriter) Insert(*pg.Bind) (string, error) {
	return "", schema.ErrNotImplemented.With("heartbeat: Insert not implemented for markFiredWriter")
}

func (w markFiredWriter) Update(bind *pg.Bind) error {
	bind.Set("fired", w.fired)
	return nil
}

// tick checks for due heartbeats, fires the callback for each, and marks them
// as fired. Errors are logged but do not abort the loop.
func (m *Manager) tick(ctx context.Context) error {
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		var list heartbeat.HeartbeatList
		if err := conn.List(ctx, &list, heartbeat.HeartbeatListRequest{Fired: types.Ptr(false)}); err != nil {
			return err
		}

		now := time.Now()
		for _, h := range list.Body {
			base := h.Created
			if h.LastFired != nil {
				base = h.LastFired.Add(time.Minute)
			}
			next := h.Schedule.Next(base)
			if next.IsZero() || next.After(now) {
				continue
			}

			futureNext := h.Schedule.Next(now.Add(time.Minute))
			var fired heartbeat.Heartbeat
			if err := conn.Update(ctx, &fired, heartbeat.HeartbeatMarkFiredSelector(h.ID), markFiredWriter{fired: futureNext.IsZero()}); err != nil {
				return err
			}
			m.onFire(ctx, &fired)
		}

		return nil
	}); err != nil {
		return pg.NormalizeError(err)
	}
	return nil
}
