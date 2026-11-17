//go:build !client

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	gofiler "github.com/mutablelogic/go-filer"
	filerfs "github.com/mutablelogic/go-filer/filer/fs"
	client "github.com/mutablelogic/go-llm/mcp/client"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	mcpserver "github.com/mutablelogic/go-llm/mcp/server"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	filer "github.com/mutablelogic/go-llm/workspace/filer"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// FilerName is the name of the workspace of the folder, which is its
	// namespace's, and its MCP server's
	FilerName = "files"

	// FilerTitle is the title of the workspace of the folder
	FilerTitle = "Files"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithFiler serves the folder, when its path is set, as an MCP server of its
// files, with the command's version, which its client is connected to, in
// memory, and connected again, should the server close its session, and
// whose namespace is added to the toolkit, so it's a workspace,
// and calls the function with the server, which it runs, or nil, when
// its path isn't set. Once the function returns, the namespace is removed,
// and the client disconnected.
func (flags FilerFlags) WithFiler(ctx server.Cmd, tools *toolkit.Toolkit, fn func(*filer.Server) error) error {
	if flags.Path == "" {
		return fn(nil)
	}
	dir, err := filepath.Abs(flags.Path)
	if err != nil {
		return err
	}
	description := fmt.Sprintf("The files in %s", dir)

	// The backend of the folder, and its server
	backend, err := filerfs.New(FilerName, dir, nil, gofiler.Opts{Tracer: ctx.Tracer()})
	if err != nil {
		return err
	}
	defer backend.Close()
	srv, err := filer.NewServer(FilerName, FilerTitle, backend,
		mcpserver.WithVersion(ctx.Version()),
		mcpserver.WithLogger(ctx.Logger()),
		mcpserver.WithTracer(ctx.Tracer()),
	)
	if err != nil {
		return err
	}

	// Its namespace, which is a workspace, in the toolkit, connected to its
	// server with a client, in memory, as the connectors' are, which is
	// connected again, should the server close its session
	ns, err := namespace.New(FilerName, description)
	if err != nil {
		return err
	}
	disconnect, err := connectFiler(ctx.Context(), ctx.Logger(), ns, description, srv.InMemoryTransport, client.WithName(ctx.Name(), ctx.Version()))
	if err != nil {
		return err
	}
	defer disconnect()
	if err := tools.Add(ns); err != nil {
		return err
	}
	defer tools.Remove(ns)
	ctx.Logger().InfoContext(ctx.Context(), "workspace added", "namespace", FilerName, "dir", dir)

	// Call the function with the server
	return fn(srv)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// reconnectDelay is how long it waits before the client is connected again,
// once the server closed its session, or it couldn't be connected
var reconnectDelay = time.Second

// connectFiler connects the namespace to the server, with a client, of the
// transport, and connects them again, should the server close its session,
// until the function it returns is called, which disconnects them
func connectFiler(ctx context.Context, logger *slog.Logger, ns *namespace.Namespace, description string, transport client.TransportFunc, opts ...client.Opt) (func(), error) {
	var c *client.Client
	closed := make(chan error, 1)
	c, err := client.New("memory://"+FilerName, append(opts, client.WithTransport(transport), client.WithChange(func(server *sdkmcp.Implementation, err error) {
		if server == nil && err != nil {
			// The server closed the session, so the namespace has no tools,
			// until it's connected again
			ns.Disconnect(c)
			select {
			case closed <- err:
			default:
			}
		}
	}))...)
	if err != nil {
		return nil, err
	}
	var serial uint64 = 1
	if err := c.Connect(ctx); err != nil {
		return nil, err
	} else if err := ns.Connect(ctx, c, description, nil, serial); err != nil {
		_ = c.Disconnect()
		return nil, err
	}

	// Connect again, once it's closed, until it's disconnected
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-closed:
				logger.WarnContext(ctx, "workspace disconnected", "namespace", FilerName, "error", err)
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(reconnectDelay):
				}
				serial++
				if err := c.Connect(ctx); err != nil {
					logger.WarnContext(ctx, "workspace not connected", "namespace", FilerName, "error", err)
				} else if err := ns.Connect(ctx, c, description, nil, serial); err != nil {
					logger.WarnContext(ctx, "workspace not connected", "namespace", FilerName, "error", err)
					_ = c.Disconnect()
				} else {
					logger.InfoContext(ctx, "workspace connected", "namespace", FilerName)
					break
				}
			}
		}
	})
	return func() {
		cancel()
		wg.Wait()
		_ = c.Disconnect()
	}, nil
}
