package toolkit

import (
	"context"
	"log/slog"

	// Packages
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sessionKey is the context key used to store the per-call Session.
type sessionKey struct{}

// Session provides contextual services for tool execution, such as logging
// and distributed tracing.
type session struct {
	id     string
	logger *slog.Logger
	meta   map[string]any
}

var _ Session = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (tk *toolkit) newSession(id, name string, meta map[string]any) Session {
	session := new(session)

	// Set up session defaults
	session.id = id
	session.logger = tk.logger.With("id", id, "name", name)
	session.meta = meta

	// Return the session. The toolkit injects this into the context for tool handlers to use.
	return session
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// SessionFromContext returns the Session injected into ctx for the current
// tool call.
func SessionFromContext(ctx context.Context) Session {
	if s, ok := ctx.Value(sessionKey{}).(Session); ok {
		return s
	}
	return &session{
		logger: slog.Default(),
		meta:   make(map[string]any),
	}
}

// WithSessionContext returns a new context with the given Session attached.
// Servers use this to inject session context before invoking tool handlers.
func WithSessionContext(ctx context.Context, s Session) context.Context {
	if s == nil {
		return ctx
	}
	return context.WithValue(ctx, sessionKey{}, s)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SESSION

func (s *session) ID() string {
	return s.id
}

func (s *session) ClientInfo() *mcp.Implementation {
	return nil
}

func (s *session) Capabilities() *mcp.ClientCapabilities {
	return nil
}

func (s *session) Meta() map[string]any {
	return s.meta
}

func (s *session) Logger() *slog.Logger {
	return s.logger
}

func (s *session) Progress(progress, total float64, message string) error {
	// In the default implementation, we don't have a client to send progress updates to, so we'll just log it.
	s.logger.Info("progress update", "progress", progress, "total", total, "message", message)

	// Return success
	return nil
}
