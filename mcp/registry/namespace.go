package registry

import (
	"context"
	"errors"
	"log/slog"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// namespaceAttempts is how many namespaces are tried, when the one which is
// made is taken, as another is added to the toolkit at once
const namespaceAttempts = 5

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// addNamespace adds the namespace of the tools of the connector's server,
// which the client is connected to, to the toolkit, when the registry has
// one, in place of the one it had, with the connector's name, as
// schema.Namespace makes it, which isn't one of the toolkit's, described by
// the server's title, or name, within ConnectTimeout
func (r *Registry) addNamespace(name string, c *client.Client, server *sdkmcp.Implementation) {
	if r.toolkit == nil {
		return
	}
	r.removeNamespace(name)
	ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
	defer cancel()
	description := server.Title
	if description == "" {
		description = server.Name
	}
	for range namespaceAttempts {
		ns, err := namespace.New(ctx, schema.Namespace(name, r.toolkit.HasNamespace), description, c)
		if err != nil {
			r.log().WarnContext(ctx, "connector's tools not listed", "connector", name, "error", err.Error())
			return
		} else if err := r.toolkit.Add(ns); errors.Is(err, llm.ErrConflict) {
			// Another took its name, as it was made
			continue
		} else if err != nil {
			r.log().WarnContext(ctx, "connector's tools not added", "connector", name, "namespace", ns.Name(), "error", err.Error())
			return
		}

		// Keep it, unless the client was replaced, or disconnected, as it
		// was added, when it's removed
		r.mu.Lock()
		current := r.clients[name].client == c && c.Connected()
		if current {
			r.namespaces[name] = ns
		}
		r.mu.Unlock()
		if !current {
			_ = r.toolkit.Remove(ns)
			return
		}
		r.log().InfoContext(ctx, "connector's tools added", "connector", name, "namespace", ns.Name(), "tools", len(ns.Tools()))
		return
	}
	r.log().WarnContext(ctx, "connector's tools not added, as their namespace was taken", "connector", name)
}

// removeNamespace removes the namespace of the tools of the connector's
// server from the toolkit, when it has one
func (r *Registry) removeNamespace(name string) {
	r.mu.Lock()
	ns, exists := r.namespaces[name]
	delete(r.namespaces, name)
	r.mu.Unlock()
	if exists && r.toolkit != nil {
		_ = r.toolkit.Remove(ns)
	}
}

// refreshNamespace lists the tools of the connector's server again, in the
// background, as it said they changed, so the toolkit has them as they are
func (r *Registry) refreshNamespace(name string) {
	r.mu.Lock()
	ns := r.namespaces[name]
	r.mu.Unlock()
	if ns == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
		defer cancel()
		if err := ns.Refresh(ctx); err != nil {
			r.log().WarnContext(ctx, "connector's tools not listed again", "connector", name, "error", err.Error())
		}
	}()
}

// namespaceOf returns the name of the namespace of the tools of the
// connector's server, or nothing, when it has none
func (r *Registry) namespaceOf(name string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ns := r.namespaces[name]; ns != nil {
		return ns.Name()
	}
	return ""
}

// log returns Run's logger, or one which logs nothing, before it runs
func (r *Registry) log() *slog.Logger {
	if logger := r.logger.Load(); logger != nil {
		return logger
	}
	return slog.New(slog.DiscardHandler)
}
