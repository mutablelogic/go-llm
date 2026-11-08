// Package registry has the connectors, which are remote MCP servers, whose
// definitions are kept in a store, with a client of each, which Run
// connects to its server
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
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
	r := &Registry{clients: make(map[string]entry)}
	if err := r.opt.apply(opts); err != nil {
		return nil, err
	}
	return r, nil
}

// Close disconnects every connector's server, such as when the process
// stops, and returns their errors
func (r *Registry) Close() error {
	r.mu.Lock()
	clients := make([]*client.Client, 0, len(r.clients))
	for _, e := range r.clients {
		clients = append(clients, e.client)
	}
	r.clients = make(map[string]entry)
	r.mu.Unlock()

	var errs []error
	for _, c := range clients {
		errs = append(errs, c.Disconnect())
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
// one it replaces
func (r *Registry) replace(connector schema.Connector, c *client.Client) {
	r.mu.Lock()
	e, exists := r.clients[connector.Name]
	r.clients[connector.Name] = entry{serial: connector.Serial, client: c}
	r.mu.Unlock()
	if exists && e.client != c {
		_ = e.client.Disconnect()
	}
}

// current returns true when the client is the connector's, which it isn't
// once it's replaced, or forgotten
func (r *Registry) current(name string, c *client.Client) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, exists := r.clients[name]
	return exists && e.client == c
}

// forget disconnects the client of the connector with the name, and
// forgets it, as the connector was changed, or deleted
func (r *Registry) forget(name string) {
	r.mu.Lock()
	e, exists := r.clients[name]
	delete(r.clients, name)
	r.mu.Unlock()
	if exists {
		_ = e.client.Disconnect()
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
	}
	if err != nil {
		result.Error = err.Error()
	}
	return &result
}

// publish publishes the change to the connector, with the registry's
// broadcaster, which Run watches, whose row is its name, and whose data is
// its serial, unless it was deleted
func (r *Registry) publish(action eventschema.EventAction, connector schema.Connector) {
	event := eventschema.Event{Action: action, Schema: schema.Name, Table: schema.ConnectorTable, Row: connector.Name}
	if action != eventschema.EventDelete {
		if data, err := json.Marshal(schema.ConnectorEvent{Serial: connector.Serial}); err == nil {
			event.Data = data
		}
	}
	r.broadcaster.Publish(event)
}

// newClient returns a client of the connector, with the registry's options
func (r *Registry) newClient(connector schema.Connector) (*client.Client, error) {
	return client.New(connector.URL, r.clientopts...)
}
