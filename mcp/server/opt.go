package server

import (
	"log/slog"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// opts collects both the Implementation metadata and ServerOptions
// while applying functional options before the SDK server is constructed.
type opts struct {
	impl    sdkmcp.Implementation
	sdkOpts sdkmcp.ServerOptions
	tracer  trace.Tracer
}

// ServerOpt is a functional option for configuring a Server.
type ServerOpt func(*opts) error

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithTitle sets a human-readable display title for the server, shown in UIs
// that support the MCP title field.
func WithTitle(title string) ServerOpt {
	return func(o *opts) error {
		o.impl.Title = title
		return nil
	}
}

// WithWebsiteURL sets the website URL advertised in the server's Implementation
// descriptor.
func WithWebsiteURL(url string) ServerOpt {
	return func(o *opts) error {
		o.impl.WebsiteURL = url
		return nil
	}
}

// WithInstructions sets the instructions string sent to clients during
// initialization. Clients may forward this to an LLM as a system prompt hint.
func WithInstructions(instructions string) ServerOpt {
	return func(o *opts) error {
		o.sdkOpts.Instructions = instructions
		return nil
	}
}

// WithLogger sets the slog.Logger used for server activity logging.
// If not set, no logging is performed.
func WithLogger(logger *slog.Logger) ServerOpt {
	return func(o *opts) error {
		o.sdkOpts.Logger = logger
		return nil
	}
}

// WithKeepAlive sets the interval at which the server sends ping requests to
// connected clients. If a client fails to respond, its session is closed.
// A zero duration (the default) disables keepalive.
func WithKeepAlive(d time.Duration) ServerOpt {
	return func(o *opts) error {
		o.sdkOpts.KeepAlive = d
		return nil
	}
}

// WithTracer sets the OpenTelemetry tracer for distributed tracing.
// If not set, tracing is disabled.
func WithTracer(t trace.Tracer) ServerOpt {
	return func(o *opts) error {
		o.tracer = t
		return nil
	}
}
