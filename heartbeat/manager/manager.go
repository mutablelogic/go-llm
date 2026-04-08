package heartbeat

import (
	"context"
	"log/slog"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	hschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Manager owns a Store and runs a background loop that fires due heartbeats.
// Create one with New, register an OnFire callback, then call Run in a goroutine.
type Manager struct {
	store        hschema.Store
	pollInterval time.Duration
	logger       *slog.Logger
	tracer       trace.Tracer
	onFire       func(context.Context, *hschema.Heartbeat)
}

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECKS

var _ llm.Connector = (*Manager)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a Manager backed by the given store.
func New(store hschema.Store, opts ...Opt) (*Manager, error) {
	if store == nil {
		return nil, schema.ErrBadParameter.With("store is required")
	}
	m := &Manager{
		store:        store,
		pollInterval: defaultPollInterval,
		logger:       slog.Default(),
		onFire:       func(context.Context, *hschema.Heartbeat) {},
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
		addHeartbeat{mgr: m}, deleteHeartbeat{mgr: m}, listHeartbeats{mgr: m}, updateHeartbeat{mgr: m},
	}, nil
}

// ListPrompts returns nil and satisfies the llm.Connector interface.
func (m *Manager) ListPrompts(_ context.Context) ([]llm.Prompt, error) {
	return nil, nil
}

// ListResources returns nil and satisfies the llm.Connector interface.
func (m *Manager) ListResources(_ context.Context) ([]llm.Resource, error) {
	return nil, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// tick checks for due heartbeats, fires the callback for each, and marks them
// as fired.  Errors are logged but do not abort the loop.
func (m *Manager) tick(ctx context.Context) error {
	fired, err := m.store.Next(ctx)
	if err != nil {
		m.logger.ErrorContext(ctx, "heartbeat: failed to fire due heartbeats", "err", err.Error())
		return err
	}
	for _, h := range fired {
		m.onFire(ctx, h)
	}
	return nil
}
