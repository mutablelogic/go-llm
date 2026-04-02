package toolkit

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	// Packages
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sessionKey is the context key used to store the per-call Session.
type sessionKey struct{}
type metaKey struct{}

// Session provides contextual services for tool execution, such as logging
// and distributed tracing.
type session struct {
	id     string
	logger *slog.Logger
	meta   []schema.MetaValue
}

var _ Session = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (tk *toolkit) newSession(name string, meta ...schema.MetaValue) Session {
	session := new(session)

	// Set up session defaults
	if v := schema.MetaForKey(meta, "id"); v != nil {
		session.id = fmt.Sprint(v)
	}
	session.logger = tk.logger.With("id", session.id, "name", name)
	session.meta = slices.Clone(meta)

	// Return the session. The toolkit injects this into the context for tool handlers to use.
	return session
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONTEXT

// SessionFromContext returns the Session injected into ctx for the current
// tool call.
func SessionFromContext(ctx context.Context) Session {
	if s, ok := ctx.Value(sessionKey{}).(Session); ok {
		return s
	}
	return &session{
		logger: slog.Default(),
	}
}

// WithSession returns a new context with the given session ID and metadata attached.
// The session ID is stored as a MetaValue with key "id" alongside any provided metadata.
func WithSession(ctx context.Context, id string, meta ...schema.MetaValue) context.Context {
	return context.WithValue(ctx, metaKey{}, append(meta, schema.Meta("id", id)))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - CONTEXT

// WithSessionContext returns a new context with the given Session attached.
func withSessionContext(ctx context.Context, s Session) context.Context {
	if s == nil {
		return ctx
	}
	return context.WithValue(ctx, sessionKey{}, s)
}

// metaFromContext returns the meta information injected into ctx
func metaFromContext(ctx context.Context) []schema.MetaValue {
	if meta, ok := ctx.Value(metaKey{}).([]schema.MetaValue); ok {
		return meta
	}
	return []schema.MetaValue{}
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
	return schema.MetaMap(s.meta)
}

func (s *session) Logger() *slog.Logger {
	return s.logger
}

func (s *session) Progress(progress, total float64, message ...string) error {
	// Error if message is too long
	if len(message) > 1 {
		return schema.ErrBadParameter.Withf("too many message arguments: expected at most one, got %d", len(message))
	}

	// In the default implementation, we don't have a client to send progress updates to, so we'll just log it.
	if len(message) == 1 {
		s.logger.Info(message[0], "progress", progress, "total", total)
	} else {
		s.logger.Info("progress update", "progress", progress, "total", total)
	}

	// Return success
	return nil
}

func (s *session) String() string {
	type json struct {
		ID   string         `json:"id,omitempty"`
		Meta map[string]any `json:"meta,omitempty"`
	}
	return types.Stringify(json{
		ID:   s.id,
		Meta: schema.MetaMap(s.meta),
	})
}
