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

// ensureNamespace adds the namespace of the connector's tools to the
// toolkit, when the registry has one, and it hasn't one already, which it
// has as long as the connector exists, with no tools, until it's connected,
// and whose name is the connector's, as schema.Namespace makes it, which
// isn't one of the toolkit's, described by its URL, until its server says
// its title. Returns it, or nil, when there's no toolkit, or it couldn't be
// added.
func (r *Registry) ensureNamespace(connector schema.Connector) *namespace.Namespace {
	if r.toolkit == nil {
		return nil
	}
	r.mu.Lock()
	ns, exists := r.namespaces[connector.Name]
	r.mu.Unlock()
	if exists {
		return ns
	}
	for range namespaceAttempts {
		ns, err := namespace.New(schema.Namespace(connector.Name, r.toolkit.HasNamespace), connector.URL, namespace.WithTimeout(r.toolTimeout), namespace.WithAgents(r.runAgent))
		if err != nil {
			r.log().Warn("connector's namespace not made", "connector", connector.Name, "error", err.Error())
			return nil
		} else if err := r.toolkit.Add(ns); errors.Is(err, llm.ErrConflict) {
			// Another took its name, as it was made
			continue
		} else if err != nil {
			r.log().Warn("connector's namespace not added", "connector", connector.Name, "namespace", ns.Name(), "error", err.Error())
			return nil
		}

		// Keep it, unless another was kept, as it was added, when it's
		// removed, and that's the connector's
		r.mu.Lock()
		kept, exists := r.namespaces[connector.Name]
		if !exists {
			r.namespaces[connector.Name] = ns
		}
		r.mu.Unlock()
		if exists {
			_ = r.toolkit.Remove(ns)
			return kept
		}
		return ns
	}
	r.log().Warn("connector's namespace not added, as its name was taken", "connector", connector.Name)
	return nil
}

// connectNamespace lists the tools of the connector's server, which the
// client is connected to, in its namespace, which are included by its
// patterns, described by the server's title, or name, within its connect
// timeout, unless it's been connected for a later definition
func (r *Registry) connectNamespace(c *client.Client, server *sdkmcp.Implementation, connector schema.Connector) {
	name := connector.Name
	ns := r.ensureNamespace(connector)
	if ns == nil {
		return
	}
	match, err := connector.Matcher()
	if err != nil {
		r.log().Warn("connector's tools not listed", "connector", name, "error", err.Error())
		return
	}
	description := server.Title
	if description == "" {
		description = server.Name
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.connectTimeout)
	defer cancel()
	if err := ns.Connect(ctx, c, description, match, connector.Serial); err != nil {
		r.log().WarnContext(ctx, "connector's tools not listed", "connector", name, "error", err.Error())
		return
	}
	r.log().InfoContext(ctx, "connector's tools added", "connector", name, "namespace", ns.Name(), "tools", len(ns.Tools()))
}

// disconnectNamespace clears the tools of the connector's namespace, as the
// client is disconnected from its server, which it keeps, with no tools,
// unless they're another client's, such as one which replaced it
func (r *Registry) disconnectNamespace(name string, c *client.Client) {
	r.mu.Lock()
	ns := r.namespaces[name]
	r.mu.Unlock()
	if ns != nil {
		ns.Disconnect(c)
	}
}

// removeNamespace removes the namespace of the connector's tools from the
// toolkit, as the connector is deleted, when it has one
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
		ctx, cancel := context.WithTimeout(context.Background(), r.connectTimeout)
		defer cancel()
		if err := ns.Refresh(ctx); err != nil {
			r.log().WarnContext(ctx, "connector's tools not listed again", "connector", name, "error", err.Error())
		}
	}()
}

// namespaceOf returns the name of the namespace of the connector's tools,
// or nothing, when it has none
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
