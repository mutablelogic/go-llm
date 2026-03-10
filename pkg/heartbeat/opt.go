package heartbeat

import (
	"context"
	"log/slog"
	"time"

	// Packages
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for configuring a Manager.
type Opt func(*Manager) error

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithPollInterval sets how often the Manager polls for due heartbeats.
// The default is 10 s.
func WithPollInterval(d time.Duration) Opt {
	return func(m *Manager) error {
		if d > 0 {
			m.pollInterval = d
		}
		return nil
	}
}

// WithLogger sets the logger used for error reporting inside the run loop.
func WithLogger(l *slog.Logger) Opt {
	return func(m *Manager) error {
		if l != nil {
			m.logger = l
		}
		return nil
	}
}

// WithOnFire registers a callback invoked for each heartbeat as it matures.
// Only one callback can be active; later calls overwrite earlier ones.
func WithOnFire(fn func(context.Context, *Heartbeat)) Opt {
	return func(m *Manager) error {
		if fn != nil {
			m.onFire = fn
		}
		return nil
	}
}

// WithTracer sets the OpenTelemetry tracer for distributed tracing.
func WithTracer(t trace.Tracer) Opt {
	return func(m *Manager) error {
		if t != nil {
			m.tracer = t
		}
		return nil
	}
}
