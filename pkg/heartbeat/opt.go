package heartbeat

import (
	"context"
	"log/slog"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/heartbeat/schema"
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
			return llm.ErrBadParameter.With("poll interval must be positive")
		}
		m.pollInterval = d
		return nil
	}
}

// WithLogger sets the logger used for error reporting inside the run loop.
func WithLogger(l *slog.Logger) Opt {
	return func(m *Manager) error {
		if l == nil {
			return llm.ErrBadParameter.With("nil logger")
		}
		m.logger = l
		return nil
	}
}

// WithOnFire registers a callback invoked for each heartbeat as it matures.
// Only one callback can be active; later calls overwrite earlier ones.
func WithOnFire(fn func(context.Context, *schema.Heartbeat)) Opt {
	return func(m *Manager) error {
		if fn == nil {
			return llm.ErrBadParameter.With("nil callback")
		}
		m.onFire = fn
		return nil
	}
}
