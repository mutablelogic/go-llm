package server

import (
	"context"
	"log/slog"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sessionKey is the context key used to store the per-call Session.
type sessionKey struct{}

// Session is available inside every tool call via SessionFromContext. It
// provides logging and progress reporting back to the connected MCP client,
// as well as read-only metadata about the client.
type Session interface {
	// ID returns the unique identifier for this client session.
	ID() string

	// ClientInfo returns the name and version of the connected client, as
	// reported during the MCP handshake. May return nil if unavailable.
	ClientInfo() *sdkmcp.Implementation

	// Capabilities returns the capabilities advertised by the client during
	// the MCP handshake. May return nil if unavailable.
	Capabilities() *sdkmcp.ClientCapabilities

	// Meta returns the _meta map sent by the client in this tool call.
	// Returns nil when no _meta was provided.
	Meta() map[string]any

	// Logger returns a slog.Logger whose output is forwarded to the client
	// as MCP notifications/message events.
	Logger() *slog.Logger

	// Progress sends a progress notification to the client.
	// progress is the amount completed so far; total is the total expected
	// (0 means unknown); message is an optional human-readable status string.
	Progress(progress, total float64, message string) error
}

// session is the concrete, per-call implementation of Session.
type session struct {
	id           string
	clientInfo   *sdkmcp.Implementation
	capabilities *sdkmcp.ClientCapabilities
	meta         map[string]any
	logger       *slog.Logger
	progress     func(progress, total float64, message string) error
}

var _ Session = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SESSION

func (s *session) ID() string                               { return s.id }
func (s *session) ClientInfo() *sdkmcp.Implementation       { return s.clientInfo }
func (s *session) Capabilities() *sdkmcp.ClientCapabilities { return s.capabilities }
func (s *session) Meta() map[string]any                     { return s.meta }
func (s *session) Logger() *slog.Logger                     { return s.logger }
func (s *session) Progress(progress, total float64, message string) error {
	return s.progress(progress, total, message)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONTEXT

// SessionFromContext returns the Session injected into ctx for the current
// tool call. If no session is present (e.g. in unit tests that invoke Run
// directly), a no-op session backed by slog.Default() is returned.
func SessionFromContext(ctx context.Context) Session {
	if s, ok := ctx.Value(sessionKey{}).(Session); ok {
		return s
	}
	return &session{
		logger:   slog.Default(),
		progress: func(_, _ float64, _ string) error { return nil },
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// withSession injects a Session into ctx for the given ServerSession, tool
// name, progress token, and _meta map.
func withSession(ctx context.Context, ss *sdkmcp.ServerSession, loggerName string, token any, meta map[string]any) context.Context {
	logger := slog.New(sdkmcp.NewLoggingHandler(ss, &sdkmcp.LoggingHandlerOptions{
		LoggerName: loggerName,
	}))

	var progressFn func(float64, float64, string) error
	if token == nil {
		progressFn = func(_, _ float64, _ string) error { return nil }
	} else {
		progressFn = func(progress, total float64, message string) error {
			return ss.NotifyProgress(ctx, &sdkmcp.ProgressNotificationParams{
				ProgressToken: token,
				Progress:      progress,
				Total:         total,
				Message:       message,
			})
		}
	}

	var clientInfo *sdkmcp.Implementation
	var capabilities *sdkmcp.ClientCapabilities
	if p := ss.InitializeParams(); p != nil {
		clientInfo = p.ClientInfo
		capabilities = p.Capabilities
	}

	return context.WithValue(ctx, sessionKey{}, &session{
		id:           ss.ID(),
		clientInfo:   clientInfo,
		capabilities: capabilities,
		meta:         meta,
		logger:       logger,
		progress:     progressFn,
	})
}
