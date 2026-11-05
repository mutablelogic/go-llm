package toolkit

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Session is what a tool can use as it runs, from what runs it, such as an
// agent loop, which a tool gets from its context with SessionFromContext
type Session interface {
	// ID returns the identifier of the session the tool runs in, such as
	// the conversation whose model called it, or the nil UUID when it
	// isn't run in one, so a tool which asks a model, such as an agent, can
	// continue from it
	ID() uuid.UUID

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

// Logf logs what the tool run in the context is doing, as a sentence in plain
// English, such as "Getting the current weather for Berlin", which is shown
// to the user, to its session's logger, at the info level
func Logf(ctx context.Context, format string, args ...any) {
	SessionFromContext(ctx).Logger().Info(fmt.Sprintf(format, args...))
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (discard) ID() uuid.UUID {
	return uuid.Nil()
}

func (discard) Logger() *slog.Logger {
	return discardLogger
}
