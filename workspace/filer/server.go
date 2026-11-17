package filer

import (
	"context"
	"log/slog"

	// Packages
	gofiler "github.com/mutablelogic/go-filer"
	llm "github.com/mutablelogic/go-llm"
	server "github.com/mutablelogic/go-llm/mcp/server"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Server is an MCP server of a go-filer backend's folders, and files, as a
// workspace, whose name, and title, are its namespace's, which has no
// tools, yet
type Server struct {
	*server.Server
	workspace *Workspace
}

// namespace is the namespace the server is of, which has no tools, yet
type namespace struct {
	name, title string
}

var _ toolkit.Namespace = namespace{}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewServer returns an MCP server of the go-filer backend's folders, and
// files, with the name, which is an identifier, and the title, with the
// server's options, such as its version. Returns ErrBadParameter when
// the name isn't an identifier, or there's no backend.
func NewServer(name, title string, filer gofiler.Filer, opts ...server.Opt) (*Server, error) {
	if !types.IsIdentifier(name) {
		return nil, llm.ErrBadParameter.Withf("name %q isn't an identifier", name)
	}
	ws, err := New(name, filer)
	if err != nil {
		return nil, err
	}
	srv, err := server.New(namespace{name: name, title: title}, append(opts, server.WithWorkspace(ws))...)
	if err != nil {
		return nil, err
	}
	return &Server{Server: srv, workspace: ws}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Workspace returns the workspace the server serves
func (s *Server) Workspace() *Workspace {
	return s.workspace
}

// Run serves the backend's changes, as it says them, to the server's
// clients, until the context is cancelled, when it returns nil, or it
// fails, which is logged
func (s *Server) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	logger.InfoContext(ctx, "filer workspace", "name", s.workspace.name, "url", s.workspace.filer.URL().String())
	if err := s.Server.Run(ctx); err != nil && ctx.Err() == nil {
		logger.ErrorContext(ctx, "filer workspace stopped", "name", s.workspace.name, "error", err.Error())
		return err
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// NAMESPACE

func (n namespace) Name() string        { return n.name }
func (n namespace) Description() string { return n.title }
func (namespace) SystemPrompt() string  { return "" }
func (namespace) Type() string          { return toolschema.NamespaceBuiltin }
func (namespace) Tools() []llm.Tool     { return nil }
func (namespace) Watch(ctx context.Context, ch chan<- eventschema.Change) error {
	return toolkit.WatchTools(ctx, ch, nil)
}
