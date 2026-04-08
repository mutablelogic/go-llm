package server

import (
	"context"
	"log/slog"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// session is the concrete, per-call implementation of ConnectorSession.
type session struct {
	id           string
	clientInfo   *sdkmcp.Implementation
	capabilities *sdkmcp.ClientCapabilities
	meta         map[string]any
	logger       *slog.Logger
	progress     func(progress, total float64, message string) error
	tracer       trace.Tracer
}

type sessionKey struct{}

var _ llm.ConnectorSession = (*session)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SESSION

func (s *session) ID() string {
	return s.id
}

func (s *session) ClientInfo() *sdkmcp.Implementation {
	return s.clientInfo
}

func (s *session) Capabilities() *sdkmcp.ClientCapabilities {
	return s.capabilities
}

func (s *session) Meta() map[string]any {
	return s.meta
}

func (s *session) Logger() *slog.Logger {
	return s.logger
}

func (s *session) Progress(progress, total float64, message string) error {
	return s.progress(progress, total, message)
}

func (s *session) Tracer() trace.Tracer {
	return s.tracer
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONTEXT

// SessionFromContext returns the Session injected into ctx for the current
// tool call. If no session is present (e.g. in unit tests that invoke Run
// directly), a no-op session backed by slog.Default() is returned.
func SessionFromContext(ctx context.Context) llm.ConnectorSession {
	if session, ok := ctx.Value(sessionKey{}).(llm.ConnectorSession); ok {
		return session
	}
	return &session{
		logger:   slog.Default(),
		progress: func(_, _ float64, _ string) error { return nil },
		tracer:   nil,
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// withSession injects a Session into ctx for the given ServerSession, tool
// name, progress token, _meta map, and optional tracer.
func withSession(ctx context.Context, ss *sdkmcp.ServerSession, loggerName string, token any, meta map[string]any, tracer trace.Tracer) context.Context {
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
		tracer:       tracer,
	})
}
