package toolkit

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
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

	// Progress says how the tool is getting on, as it runs, as MCP's
	// progress notification does: how far it's got, which increases each
	// time, out of the total, when it's known, or zero, and a message, such
	// as that an agent it runs is replying, which what runs it can show, in
	// place of its progress before, rather than keeping it, as a line it
	// logs is
	Progress(progress, total float64, message string)

	// Usage adds the tokens the tool used, such as when it asks a model, as
	// an agent does, to those of its result, so what runs it counts them
	Usage(llm.Usage)
}

// sessionKey is the key of the session in a context
type sessionKey struct{}

// promptSessionKey is the key of the session the system prompts of
// namespaces are for, in a context
type promptSessionKey struct{}

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

// WithPromptSession returns the context, in which the system prompts of
// namespaces are for the session with the identifier, such as the
// conversation a model is asked in, which SessionPrompter namespaces add to
func WithPromptSession(ctx context.Context, session uuid.UUID) context.Context {
	return context.WithValue(ctx, promptSessionKey{}, session)
}

// promptSession returns the session the system prompts of namespaces are
// for, in the context, or the session of the tool run in it, or the nil
// UUID, when there's neither
func promptSession(ctx context.Context) uuid.UUID {
	if session, ok := ctx.Value(promptSessionKey{}).(uuid.UUID); ok {
		return session
	}
	return SessionFromContext(ctx).ID()
}

// Progressf says how the tool run in the context is getting on, as a
// sentence in plain English, such as "Segmenting the transcript", which is
// shown to the user, in place of what it said before
func Progressf(ctx context.Context, format string, args ...any) {
	SessionFromContext(ctx).Progress(0, 0, fmt.Sprintf(format, args...))
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

func (discard) Progress(float64, float64, string) {}

func (discard) Usage(llm.Usage) {}

func (discard) Logger() *slog.Logger {
	return discardLogger
}
