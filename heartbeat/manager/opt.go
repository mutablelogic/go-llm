package heartbeat

import (
	"context"
	"log/slog"
	"time"

	// Packages
	hschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for configuring a Manager.
type Opt func(*Manager) error

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultPollInterval is how often the Manager polls the store for due heartbeats.
const defaultPollInterval = 10 * time.Second

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithPollInterval sets how often the Manager polls for due heartbeats.
// The default is 10 s.
func WithPollInterval(d time.Duration) Opt {
	return func(m *Manager) error {
		if d <= 0 {
			return schema.ErrBadParameter.With("poll interval must be positive")
		}
		m.pollInterval = d
		return nil
	}
}

// WithLogger sets the logger used for error reporting inside the run loop.
func WithLogger(l *slog.Logger) Opt {
	return func(m *Manager) error {
		if l == nil {
			return schema.ErrBadParameter.With("nil logger")
		}
		m.logger = l
		return nil
	}
}

// WithTracer sets the tracer used for tool execution spans.
func WithTracer(t trace.Tracer) Opt {
	return func(m *Manager) error {
		m.tracer = t
		return nil
	}
}

// WithOnFire registers a callback invoked for each heartbeat as it matures.
// Only one callback can be active; later calls overwrite earlier ones.
func WithOnFire(fn func(context.Context, *hschema.Heartbeat)) Opt {
	return func(m *Manager) error {
		if fn == nil {
			return schema.ErrBadParameter.With("nil callback")
		}
		m.onFire = fn
		return nil
	}
}
