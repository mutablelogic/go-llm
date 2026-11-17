package server

import (
	"log/slog"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option of a server
type Opt func(*opt) error

type opt struct {
	// The version of the server, which its clients are told, as they connect
	version string

	// Where the server logs, such as a tool which can't be served, and what
	// its tools log, as they run
	logger *slog.Logger

	// How often the server pings its clients, or zero for never
	keepAlive time.Duration

	// The tracer of the tools' calls, or nil
	tracer trace.Tracer
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (o *opt) apply(opts []Opt) error {
	o.version = schema.DefaultVersion
	o.logger = slog.New(slog.DiscardHandler)
	for _, fn := range opts {
		if err := fn(o); err != nil {
			return err
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithVersion is the version of the server, which its clients are told, as
// they connect, rather than schema.DefaultVersion
func WithVersion(version string) Opt {
	return func(o *opt) error {
		o.version = version
		return nil
	}
}

// WithLogger is where the server logs, such as a tool which can't be
// served, and what its tools log, as they run, rather than nowhere
func WithLogger(logger *slog.Logger) Opt {
	return func(o *opt) error {
		if logger == nil {
			return llm.ErrBadParameter.With("missing logger")
		}
		o.logger = logger
		return nil
	}
}

// WithKeepAlive pings each client at the interval, and closes its session,
// when it doesn't reply
func WithKeepAlive(interval time.Duration) Opt {
	return func(o *opt) error {
		if interval < 0 {
			return llm.ErrBadParameter.Withf("keep alive %v is negative", interval)
		}
		o.keepAlive = interval
		return nil
	}
}

// WithTracer traces the calls of the server's tools
func WithTracer(tracer trace.Tracer) Opt {
	return func(o *opt) error {
		o.tracer = tracer
		return nil
	}
}
