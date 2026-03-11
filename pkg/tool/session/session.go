package session

import (
	"context"
	"log/slog"

	// Packages
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sessionKey is the context key used to store the per-call Session.
type sessionKey struct{}

// Session provides contextual services for tool execution, such as logging
// and distributed tracing. Use FromContext to retrieve the session injected
// by the server, or a safe no-op default if none is present.
type Session interface {
	// Logger returns a slog.Logger for structured logging during tool execution.
	Logger() *slog.Logger

	// Tracer returns the OpenTelemetry tracer for distributed tracing.
	// May return nil if no tracer was configured.
	Tracer() trace.Tracer
}

// defaultSession is a no-op Session returned when no session is in context.
type defaultSession struct{}

var _ Session = (*defaultSession)(nil)

func (defaultSession) Logger() *slog.Logger { return slog.Default() }
func (defaultSession) Tracer() trace.Tracer { return nil }

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// FromContext returns the Session injected into ctx for the current
// tool call. If no session is present (e.g. in unit tests that invoke Run
// directly), a no-op session backed by slog.Default() is returned.
func FromContext(ctx context.Context) Session {
	if s, ok := ctx.Value(sessionKey{}).(Session); ok {
		return s
	}
	return &defaultSession{}
}

// NewContext returns a new context with the given Session attached.
// Servers use this to inject session context before invoking tool handlers.
func NewContext(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}
