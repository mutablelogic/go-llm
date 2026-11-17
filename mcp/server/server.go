// Package server is an MCP server of a toolkit namespace's tools, which
// follows the namespace's tools as they change, and is connected to with a
// transport, such as one which is in memory, for a client in the same
// process.
package server

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Server is an MCP server of a namespace's tools, whose name, title and
// instructions are the namespace's name, description and system prompt
type Server struct {
	opt
	ns     toolkit.Namespace
	server *sdkmcp.Server

	// The tools which are served, by their names, as the namespace returned
	// them, so a tool which changed is served again
	mu    sync.Mutex
	tools map[string]llm.Tool
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a server of the namespace's tools, as they are, with the
// options, such as its version, with WithVersion. Run follows its tools,
// as they change. Returns ErrBadParameter when there's no namespace.
func New(ns toolkit.Namespace, opts ...Opt) (*Server, error) {
	if ns == nil {
		return nil, llm.ErrBadParameter.With("missing namespace")
	}
	s := &Server{ns: ns, tools: make(map[string]llm.Tool)}
	if err := s.opt.apply(opts); err != nil {
		return nil, err
	}
	s.server = sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    ns.Name(),
		Title:   ns.Description(),
		Version: s.version,
	}, &sdkmcp.ServerOptions{
		Instructions: ns.SystemPrompt(),
		Logger:       s.logger,
		KeepAlive:    s.keepAlive,
	})
	for _, tool := range ns.Tools() {
		if tool != nil {
			s.sync(tool.Name())
		}
	}
	return s, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run follows the namespace's tools, as they change, so its clients are
// told, until the context is cancelled, when it returns nil, or the
// namespace's watch fails, when it returns its error
func (s *Server) Run(ctx context.Context) error {
	ch := make(chan eventschema.Event, 16)
	done := make(chan error, 1)
	go func() { done <- s.ns.Watch(ctx, ch) }()
	for {
		select {
		case event := <-ch:
			s.sync(event.Row)
		case err := <-done:
			return err
		}
	}
}

// Connect serves a session with the transport, such as stdio, until its
// client closes it, or it's closed
func (s *Server) Connect(ctx context.Context, transport sdkmcp.Transport) (*sdkmcp.ServerSession, error) {
	if transport == nil {
		return nil, llm.ErrBadParameter.With("missing transport")
	}
	return s.server.Connect(ctx, transport, nil)
}

// InMemoryTransport returns the client's end of a new transport in memory,
// whose server end is connected, so a client in the same process, such as
// one with client.WithTransport, is connected with it. Each is connected
// once, so it's called for each connection.
func (s *Server) InMemoryTransport(ctx context.Context) (sdkmcp.Transport, error) {
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	if _, err := s.Connect(ctx, serverTransport); err != nil {
		return nil, err
	}
	return clientTransport, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// sync serves the namespace's tool with the name, as Tools returns it now,
// or, when it doesn't, stops serving it, and does nothing when it's as it
// was, so the clients are told only of changes
func (s *Server) sync(name string) {
	var tool llm.Tool
	for _, t := range s.ns.Tools() {
		if t != nil && t.Name() == name {
			tool = t
			break
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	served, exists := s.tools[name]
	switch {
	case tool == nil && exists:
		delete(s.tools, name)
		s.server.RemoveTools(name)
	case tool == nil:
	case exists && same(served, tool):
	default:
		if err := s.add(tool); err != nil {
			s.logger.Warn("tool isn't served", "namespace", s.ns.Name(), "tool", name, "error", err.Error())
			if exists {
				delete(s.tools, name)
				s.server.RemoveTools(name)
			}
			return
		}
		s.tools[name] = tool
	}
}

// add serves the tool, or replaces the one with its name, or returns why it
// can't be, such as its input schema, which the SDK panics on
func (s *Server) add(tool llm.Tool) (err error) {
	sdkTool, err := sdkToolOf(tool)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	s.server.AddTool(sdkTool, s.handler(tool))
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// same returns true when the tools are the same one, which they are when
// they're equal values, which can be compared
func same(a, b llm.Tool) (result bool) {
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta != tb || ta == nil || !ta.Comparable() {
		return false
	}
	defer func() {
		if recover() != nil {
			result = false
		}
	}()
	return a == b
}
