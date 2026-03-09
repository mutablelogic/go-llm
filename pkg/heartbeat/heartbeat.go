package heartbeat

import (
	"context"
	"log/slog"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

// defaultPollInterval is how often the Manager polls the store for due heartbeats.
const defaultPollInterval = 10 * time.Second

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Manager owns a Store and runs a background loop that fires due heartbeats.
// Create one with New, register an OnFire callback, then call Run in a goroutine.
type Manager struct {
	store        *Store
	pollInterval time.Duration
	logger       *slog.Logger
	onFire       func(context.Context, *Heartbeat)
}

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECKS

var _ llm.Connector = (*Manager)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a Manager backed by the given store.
func New(store *Store, opts ...Opt) (*Manager, error) {
	if store == nil {
		return nil, llm.ErrBadParameter.With("store is required")
	}
	m := &Manager{
		store:        store,
		pollInterval: defaultPollInterval,
		logger:       slog.Default(),
		onFire:       func(context.Context, *Heartbeat) {},
	}
	for _, o := range opts {
		if err := o(m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run starts the maturity-check loop and blocks until ctx is cancelled.
// It returns nil when ctx is done and satisfies the llm.Connector interface.
func (m *Manager) Run(ctx context.Context) error {
	ticker := time.NewTicker(m.pollInterval)
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

// ListTools returns the heartbeat tools and satisfies the llm.Connector interface.
func (m *Manager) ListTools(_ context.Context) ([]llm.Tool, error) {
	return []llm.Tool{
		&addHeartbeat{mgr: m},
		&deleteHeartbeat{mgr: m},
		&listHeartbeats{mgr: m},
		&updateHeartbeat{mgr: m},
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// tick checks for due heartbeats, fires the callback for each, and marks them
// as fired.  Errors are logged but do not abort the loop.
func (m *Manager) tick(ctx context.Context) {
	due, err := m.store.Due()
	if err != nil {
		m.logger.Error("heartbeat: failed to query due heartbeats", "err", err)
		return
	}
	for _, h := range due {
		m.logger.Info("heartbeat fired", "id", h.ID, "message", h.Message)
		// Fire the callback first so last_fired in the payload reflects the
		// previous occurrence — useful for answering "when did you last remind me?".
		m.onFire(ctx, h)
		if err := m.store.MarkFired(h.ID); err != nil {
			m.logger.Error("heartbeat: failed to mark fired", "id", h.ID, "err", err)
		}
	}
}
