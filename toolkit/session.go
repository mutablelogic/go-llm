package toolkit

import (
	"context"
	"log/slog"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Session is what a tool can use as it runs, from what runs it, such as an
// agent loop, which a tool gets from its context with SessionFromContext
type Session interface {
	// Logger returns the logger the tool logs to as it runs, such as to say
	// what it's doing, which what runs it can show, such as by streaming it
	Logger() *slog.Logger
}

// sessionKey is the key of the session in a context
type sessionKey struct{}

// discard is the session of a tool which isn't run in one, whose logger
// discards what's logged
type discard struct{}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var discardLogger = slog.New(slog.DiscardHandler)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// WithSession returns the context with the session, which the tools run in
// the context get with SessionFromContext
func WithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, session)
}

// SessionFromContext returns the session of the tool run in the context, or,
// when it isn't run in one, a session whose logger discards what's logged,
// so it's never nil
func SessionFromContext(ctx context.Context) Session {
	if session, ok := ctx.Value(sessionKey{}).(Session); ok && session != nil {
		return session
	}
	return discard{}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (discard) Logger() *slog.Logger {
	return discardLogger
}
