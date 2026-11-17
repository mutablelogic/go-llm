// Package server is an MCP server of a toolkit namespace's tools, which
// follows the namespace's tools as they change, and is connected to with a
// transport, such as one which is in memory, for a client in the same
// process.
package server

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
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

	// How many of the server's subscriptions there are to each resource of
	// a workspace which is subscribed to, as it's subscribed to once, while
	// any of them are
	subscribing   sync.Mutex
	subscriptions map[string]int
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
	s := &Server{ns: ns, tools: make(map[string]llm.Tool), subscriptions: make(map[string]int)}
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
		Capabilities: s.capabilities(),
		// With a workspace which changes, a client subscribes to its
		// resources, which the SDK keeps, so it's told they're updated
		SubscribeHandler:   s.subscribe(),
		UnsubscribeHandler: s.unsubscribe(),
	})

	// Its tools are the namespace's, as they are now, each time they're
	// listed, or called, so one which was removed, before it was watched,
	// or while it isn't, isn't served
	s.server.AddReceivingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			switch method {
			case "tools/list", "tools/call":
				s.sync()
			case "resources/list":
				if s.workspace != nil {
					return s.listResources(ctx, req)
				}
			case "resources/read":
				if s.workspace != nil {
					return s.readResource(ctx, req)
				}
			}
			return next(ctx, method, req)
		}
	})
	// A workspace whose resources are under a root says so, with a resource
	// template of it, such as file:///{+path}, which is read as its other
	// resources are, above
	if rooted, ok := s.workspace.(workspace.Rooted); ok && rooted.Root() != "" {
		s.server.AddResourceTemplate(&sdkmcp.ResourceTemplate{
			URITemplate: schema.RootTemplate(rooted.Root()),
			Name:        ns.Name(),
			Title:       ns.Description(),
		}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
			return nil, sdkmcp.ResourceNotFoundError(req.Params.URI)
		})
	}
	s.sync()
	return s, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run follows the namespace's tools, as they change, so its clients are
// told, until the context is cancelled, when it returns nil, or the
// namespace's watch fails, when it returns its error. Its tools are also
// the namespace's as they're listed, or called, whether it runs or not.
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan eventschema.Change, 16)
	done := make(chan error, 2)
	go func() { done <- s.ns.Watch(ctx, ch) }()

	// Follow the workspace's changes, too, when it says them, unless it
	// can't, such as a filer whose backend doesn't watch, which is logged,
	// and it's served without them
	changes := make(chan schema.Change, 16)
	if watcher, ok := s.workspace.(workspace.Watcher); ok {
		go func() {
			err := watcher.WatchResources(ctx, changes)
			if errors.Is(err, llm.ErrNotImplemented) {
				s.logger.Info("workspace isn't watched", "namespace", s.ns.Name(), "reason", err.Error())
				return
			}
			done <- err
		}()
	}
	s.sync()
	watching := false
	for {
		select {
		case <-ch:
			s.sync()
		case change := <-changes:
			// The first change says the workspace is watching, so a client
			// which listed its resources before then, and may have missed
			// a change, is told to list them again
			if !watching {
				watching = true
				s.listChanged()
				continue
			}
			s.changed(ctx, change)
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

// sync serves the namespace's tools, as Tools returns them now, and stops
// serving those it doesn't, and does nothing for those which are as they
// were, so the clients are told only of changes
func (s *Server) sync() {
	current := make(map[string]llm.Tool)
	for _, tool := range s.ns.Tools() {
		if tool != nil {
			if _, exists := current[tool.Name()]; !exists {
				current[tool.Name()] = tool
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	var removed []string
	for name := range s.tools {
		if _, exists := current[name]; !exists {
			removed = append(removed, name)
			delete(s.tools, name)
		}
	}
	if len(removed) > 0 {
		s.server.RemoveTools(removed...)
	}
	for name, tool := range current {
		if served, exists := s.tools[name]; exists && same(served, tool) {
			continue
		}
		if err := s.add(tool); err != nil {
			s.logger.Warn("tool isn't served", "namespace", s.ns.Name(), "tool", name, "error", err.Error())
			if _, exists := s.tools[name]; exists {
				delete(s.tools, name)
				s.server.RemoveTools(name)
			}
			continue
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
