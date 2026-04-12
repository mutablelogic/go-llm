package heartbeat

import (
	"context"
	"log/slog"
	"time"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat/schema"
	kernel "github.com/mutablelogic/go-llm/kernel/schema"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for configuring a Manager.
type Opt func(*opts) error

// opt combines all configuration options for the heartbeat manager.
type opts struct {
	schema, llmschema string
	interval          time.Duration
	logger            *slog.Logger
	tracer            trace.Tracer
	onFire            func(context.Context, *heartbeat.Heartbeat)
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opts) apply(opts ...Opt) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(o); err != nil {
			return err
		}
	}
	return nil
}

func (o *opts) defaults() {
	o.schema = heartbeat.DefaultSchema
	o.llmschema = kernel.DefaultSchema
	o.interval = defaultPollInterval
	o.logger = slog.Default()
	o.onFire = func(context.Context, *heartbeat.Heartbeat) {}
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultPollInterval is how often the Manager polls for due heartbeats.
const defaultPollInterval = 10 * time.Second

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithSchema sets the heartbeat and llm database schema names used by the manager.
func WithSchema(heartbeat, llm string) Opt {
	return func(o *opts) error {
		if heartbeat != "" {
			o.schema = heartbeat
		}
		if llm != "" {
			o.llmschema = llm
		}
		return nil
	}
}

// WithInterval sets how often the Manager polls for due heartbeats.
// The default is 10 s.
func WithInterval(d time.Duration) Opt {
	return func(o *opts) error {
		if d <= 0 {
			return kernel.ErrBadParameter.With("poll interval must be positive")
		}
		o.interval = d
		return nil
	}
}

// WithPollInterval sets how often the Manager polls for due heartbeats.
func WithPollInterval(d time.Duration) Opt {
	return WithInterval(d)
}

// WithLogger sets the logger used for error reporting inside the run loop.
func WithLogger(l *slog.Logger) Opt {
	return func(o *opts) error {
		if l == nil {
			return kernel.ErrBadParameter.With("nil logger")
		}
		o.logger = l
		return nil
	}
}

// WithTracer sets the tracer used for tool execution spans.
func WithTracer(t trace.Tracer) Opt {
	return func(o *opts) error {
		o.tracer = t
		return nil
	}
}

// WithOnFire registers a callback invoked for each heartbeat as it matures.
// Only one callback can be active; later calls overwrite earlier ones.
func WithOnFire(fn func(context.Context, *heartbeat.Heartbeat)) Opt {
	return func(o *opts) error {
		if fn == nil {
			return kernel.ErrBadParameter.With("nil callback")
		}
		o.onFire = fn
		return nil
	}
}
