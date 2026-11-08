// Package registry has the connectors, which are remote MCP servers, whose
// definitions are kept in a store, with a client of each, which Run
// connects to its server
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Registry has the connectors, whose definitions are kept in its store,
// which is read each time they're asked for, so they're as the store has
// them, and a client of each, which Run connects, which is created once,
// and again once the connector's definition changes, when the session of
// the one it replaces is closed
type Registry struct {
	opt

	mu      sync.Mutex
	clients map[string]entry

	// The namespaces of the connectors' servers' tools, in the toolkit, by
	// the connectors' names, once they're connected, and the logger of what
	// happens to them, which is Run's
	namespaces map[string]*namespace.Namespace
	logger     atomic.Pointer[slog.Logger]

	// The registry's own broadcaster, of the connectors' changes, as they're
	// created, updated, or deleted, which Run watches, so it connects, or
	// disconnects, them at once
	events *broadcaster.Broadcaster
}

// entry is the client of a connector, and the serial of the definition it was
// created with
type entry struct {
	serial uint64
	client *client.Client
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a registry, with the options, such as its store, with
// WithStore, which keeps its connectors in memory otherwise
func New(opts ...Opt) (*Registry, error) {
	r := &Registry{clients: make(map[string]entry), namespaces: make(map[string]*namespace.Namespace), events: broadcaster.New()}
	if err := r.opt.apply(opts); err != nil {
		return nil, err
	}
	return r, nil
}

// Close disconnects every connector's server, such as when the process
// stops, and returns their errors
func (r *Registry) Close() error {
	r.mu.Lock()
	clients := r.clients
	r.clients = make(map[string]entry)
	r.mu.Unlock()

	var errs []error
	for name, e := range clients {
		errs = append(errs, r.disconnect(name, e))
	}
	return errors.Join(errs...)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONNECTORS

// ListConnectors returns the connectors, with their status, in order of
// their names
func (r *Registry) ListConnectors(ctx context.Context) (*schema.ConnectorList, error) {
	connectors, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(connectors, func(a, b schema.Connector) int { return strings.Compare(a.Name, b.Name) })
	list := &schema.ConnectorList{Count: uint(len(connectors)), Body: make([]*schema.Connector, 0, len(connectors))}
	for _, connector := range connectors {
		list.Body = append(list.Body, r.status(connector))
	}
	return list, nil
}

// GetConnector returns the connector with the name, with its status, or
// ErrNotFound when there's none
func (r *Registry) GetConnector(ctx context.Context, name string) (*schema.Connector, error) {
	connector, err := r.get(ctx, name)
	if err != nil {
		return nil, err
	}
	return r.status(connector), nil
}

// CreateConnector creates the connector, and returns it, with its serial.
// It's connected by Run. Returns ErrBadParameter when its name, or its fields,
// aren't valid, and ErrConflict when there's a connector with its name.
func (r *Registry) CreateConnector(ctx context.Context, connector schema.Connector) (*schema.Connector, error) {
	connector = connector.Definition()
	connector.Serial = 0
	if err := connector.Validate(); err != nil {
		return nil, err
	} else if _, err := r.newClient(connector); err != nil {
		return nil, err
	}
	created, err := r.store.Put(ctx, connector)
	if err != nil {
		return nil, err
	}
	r.publish(eventschema.EventCreate, created)
	return r.status(created), nil
}

// UpdateConnector replaces the fields of the connector with the name, once
// its server, as it is now, is connected, within ConnectTimeout, so a URL
// which isn't a server's isn't stored, and returns it, with its new serial,
// and connected. Its server, as it was, is disconnected, once it's
// replaced. Returns ErrNotFound when there's no connector with the name,
// ErrBadParameter when the fields aren't valid, ErrServiceUnavailable when
// its server, as it is now, can't be connected, when it's left as it was,
// and ErrConflict when it was changed as it was updated.
func (r *Registry) UpdateConnector(ctx context.Context, name string, meta schema.ConnectorMeta) (*schema.Connector, error) {
	connector, err := r.get(ctx, name)
	if err != nil {
		return nil, err
	}
	connector.ConnectorMeta = meta
	connector = connector.Definition()
	if err := connector.Validate(); err != nil {
		return nil, err
	}

	// Connect to its server, as it is now, before it's stored
	c, err := r.newClient(connector)
	if err != nil {
		return nil, err
	}
	connectCtx, cancel := context.WithTimeout(ctx, ConnectTimeout)
	defer cancel()
	if err := c.Connect(connectCtx); err != nil {
		return nil, err
	}

	// Store it, and replace its client, as it was, with the one which is
	// connected, or disconnect it, when it couldn't be stored
	updated, err := r.store.Put(ctx, connector)
	if err != nil {
		_ = c.Disconnect()
		return nil, err
	}
	r.replace(updated, c)
	r.publish(eventschema.EventUpdate, updated)
	if server, _ := c.Status(); server != nil {
		// It was connected before it was the connector's, so it's said now
		r.connection(updated, c, server, nil)
	}
	return r.status(updated), nil
}

// DeleteConnector deletes the connector with the name, and disconnects its
// server, when it's connected, and returns it, as it was. Returns
// ErrNotFound when there's no connector with the name, and ErrConflict when
// it was changed as it was deleted.
func (r *Registry) DeleteConnector(ctx context.Context, name string) (*schema.Connector, error) {
	connector, err := r.get(ctx, name)
	if err != nil {
		return nil, err
	} else if err := r.store.Delete(ctx, name, connector.Serial); err != nil {
		return nil, err
	}
	r.forget(name)
	r.publish(eventschema.EventDelete, connector)
	result := connector.Definition()
	return &result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// get returns the connector with the name, as the store has it, or
// ErrNotFound when there's none
func (r *Registry) get(ctx context.Context, name string) (schema.Connector, error) {
	connectors, err := r.store.List(ctx)
	if err != nil {
		return schema.Connector{}, err
	}
	for _, connector := range connectors {
		if connector.Name == name {
			return connector, nil
		}
	}
	return schema.Connector{}, llm.ErrNotFound.Withf("connector %q", name)
}

// client returns the client of the connector, as it is, which is created
// once, and again once its definition changes, when the one it replaces is
// disconnected
func (r *Registry) client(connector schema.Connector) (*client.Client, error) {
	r.mu.Lock()
	e, exists := r.clients[connector.Name]
	if exists && e.serial == connector.Serial {
		r.mu.Unlock()
		return e.client, nil
	}
	c, err := r.newClient(connector)
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	r.clients[connector.Name] = entry{serial: connector.Serial, client: c}
	r.mu.Unlock()
	if exists {
		_ = e.client.Disconnect()
	}
	return c, nil
}

// replace makes the client the connector's, as it is, and disconnects the
// one it replaces, which isn't said, as the client which replaces it is
// connected
func (r *Registry) replace(connector schema.Connector, c *client.Client) {
	r.mu.Lock()
	e, exists := r.clients[connector.Name]
	r.clients[connector.Name] = entry{serial: connector.Serial, client: c}
	r.mu.Unlock()
	if exists && e.client != c {
		_ = e.client.Disconnect()
	}
}

// disconnect disconnects the client of the connector with the name, which
// is no longer the connector's, and says it was, when it was connected
func (r *Registry) disconnect(name string, e entry) error {
	connected := e.client.Connected()
	err := e.client.Disconnect()
	if connected {
		r.connection(schema.Connector{Name: name, Serial: e.serial}, e.client, nil, nil)
	}
	return err
}

// current returns true when the client is the connector's, which it isn't
// once it's replaced, or forgotten
func (r *Registry) current(name string, c *client.Client) bool {
	_, ok := r.serial(name, c)
	return ok
}

// serial returns the serial of the connector, as it is, whose client is
// the client, or false when it isn't the connector's, such as once it's
// replaced, or forgotten
func (r *Registry) serial(name string, c *client.Client) (uint64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, exists := r.clients[name]
	if !exists || e.client != c {
		return 0, false
	}
	return e.serial, true
}

// forget disconnects the client of the connector with the name, and
// forgets it, as the connector was deleted, or another process deleted it
func (r *Registry) forget(name string) {
	r.mu.Lock()
	e, exists := r.clients[name]
	delete(r.clients, name)
	r.mu.Unlock()
	if exists {
		_ = r.disconnect(name, e)
	}
}

// status returns the connector, with whether its server is connected, and
// which it is, or why it couldn't be connected, by its client, when it has
// one, for the connector as it is
func (r *Registry) status(connector schema.Connector) *schema.Connector {
	result := connector.Definition()
	r.mu.Lock()
	e, exists := r.clients[connector.Name]
	r.mu.Unlock()
	if !exists || e.serial != connector.Serial {
		return &result
	}
	server, err := e.client.Status()
	if server != nil {
		result.Connected = true
		result.Server = &schema.ConnectorServer{Name: server.Name, Version: server.Version}
		result.Namespace = r.namespaceOf(connector.Name)
	}
	if err != nil {
		result.Error = err.Error()
	}
	return &result
}

// publish publishes the change to the connector, with the registry's own
// broadcaster, which Run watches, whose row is its name, and whose data is
// its serial, unless it was deleted
func (r *Registry) publish(action eventschema.EventAction, connector schema.Connector) {
	event := eventschema.Event{Action: action, Schema: schema.Name, Table: schema.ConnectorTable, Row: connector.Name}
	if action != eventschema.EventDelete {
		if data, err := json.Marshal(schema.ConnectorEvent{Serial: connector.Serial}); err == nil {
			event.Data = data
		}
	}
	r.events.Publish(event)
}

// connection adds the namespace of the tools of the connector's server, as
// the client is connected to it, to the toolkit, or removes it, as it's
// disconnected, and publishes that it was connected, with which server it
// is, as it said, or disconnected, with why, when it wasn't disconnected,
// with the broadcaster the registry was given, when it was, as a connector
// which is created, or deleted
func (r *Registry) connection(connector schema.Connector, c *client.Client, server *sdkmcp.Implementation, err error) {
	if server != nil {
		r.addNamespace(connector.Name, c, server)
	} else {
		r.removeNamespace(connector.Name)
	}
	if r.broadcaster == nil {
		return
	}
	data := schema.ConnectorEvent{Serial: connector.Serial}
	action := eventschema.EventDelete
	if server != nil {
		action = eventschema.EventCreate
		data.Server = &schema.ConnectorServer{Name: server.Name, Version: server.Version}
	} else if err != nil {
		data.Error = err.Error()
	}
	event := eventschema.Event{Action: action, Schema: schema.Name, Table: schema.ConnectorTable, Row: connector.Name}
	if raw, err := json.Marshal(data); err == nil {
		event.Data = raw
	}
	r.broadcaster.Publish(event)
}

// newClient returns a client of the connector, with the registry's options,
// which says when it's connected, or disconnected, while it's the
// connector's
func (r *Registry) newClient(connector schema.Connector) (*client.Client, error) {
	var c *client.Client
	opts := append(slices.Clone(r.clientopts), client.WithChange(func(server *sdkmcp.Implementation, err error) {
		if serial, ok := r.serial(connector.Name, c); ok {
			r.connection(schema.Connector{Name: connector.Name, Serial: serial}, c, server, err)
		}
	}), client.WithToolsChanged(func() {
		if r.current(connector.Name, c) {
			r.refreshNamespace(connector.Name)
		}
	}))
	var err error
	c, err = client.New(connector.URL, opts...)
	return c, err
}
