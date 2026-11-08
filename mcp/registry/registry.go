// Package registry has the connectors, which are remote MCP servers, whose
// definitions are kept in a store, with a client of each
package registry

import (
	"context"
	"encoding/json"
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
// them, and a client of each, which is created once, and again once the
// connector's definition changes
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

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SERVERS

// ListConnectors returns the connectors, in order of their names
func (r *Registry) ListConnectors(ctx context.Context) (*schema.ConnectorList, error) {
	connectors, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(connectors, func(a, b schema.Connector) int { return strings.Compare(a.Name, b.Name) })
	return &schema.ConnectorList{Count: uint(len(connectors)), Body: connectors}, nil
}

// GetConnector returns the connector with the name, or ErrNotFound when there's
// none
func (r *Registry) GetConnector(ctx context.Context, name string) (schema.Connector, error) {
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

// CreateConnector creates the connector, and returns it, with its serial.
// Returns ErrBadParameter when its name, or its fields, aren't valid, and
// ErrConflict when there's a connector with its name.
func (r *Registry) CreateConnector(ctx context.Context, connector schema.Connector) (schema.Connector, error) {
	connector.Serial = 0
	if err := connector.Validate(); err != nil {
		return schema.Connector{}, err
	} else if _, err := r.newClient(connector); err != nil {
		return schema.Connector{}, err
	}
	created, err := r.store.Put(ctx, connector)
	if err != nil {
		return schema.Connector{}, err
	}
	r.publish(eventschema.EventCreate, created)
	return created, nil
}

// UpdateConnector replaces the fields of the connector with the name, and returns
// it, with its new serial, once its client is created again, when it's
// next asked for. Returns ErrNotFound when there's no connector with the name,
// ErrBadParameter when the fields aren't valid, and ErrConflict when it was
// changed as it was updated.
func (r *Registry) UpdateConnector(ctx context.Context, name string, meta schema.ConnectorMeta) (schema.Connector, error) {
	connector, err := r.GetConnector(ctx, name)
	if err != nil {
		return schema.Connector{}, err
	}
	connector.ConnectorMeta = meta
	if err := connector.Validate(); err != nil {
		return schema.Connector{}, err
	} else if _, err := r.newClient(connector); err != nil {
		return schema.Connector{}, err
	}
	updated, err := r.store.Put(ctx, connector)
	if err != nil {
		return schema.Connector{}, err
	}
	r.publish(eventschema.EventUpdate, updated)
	return updated, nil
}

// DeleteConnector deletes the connector with the name, and its client, and
// returns it, as it was. Returns ErrNotFound when there's no connector with the
// name, and ErrConflict when it was changed as it was deleted.
func (r *Registry) DeleteConnector(ctx context.Context, name string) (schema.Connector, error) {
	connector, err := r.GetConnector(ctx, name)
	if err != nil {
		return schema.Connector{}, err
	} else if err := r.store.Delete(ctx, name, connector.Serial); err != nil {
		return schema.Connector{}, err
	}
	r.mu.Lock()
	delete(r.clients, name)
	r.mu.Unlock()
	r.publish(eventschema.EventDelete, connector)
	return connector, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CLIENTS

// Client returns the client of the MCP server of the connector with the
// name, which is created once, and again once the connector's definition
// changes, or ErrNotFound when there's no connector with the name. It isn't
// connected.
func (r *Registry) Client(ctx context.Context, name string) (*client.Client, error) {
	connector, err := r.GetConnector(ctx, name)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, exists := r.clients[name]; exists && e.serial == connector.Serial {
		return e.client, nil
	}
	c, err := r.newClient(connector)
	if err != nil {
		return nil, err
	}
	r.clients[name] = entry{serial: connector.Serial, client: c}
	return c, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// publish publishes the change to the connector, when the registry has a
// broadcaster, whose row is its name, and whose data is its serial, unless
// it was deleted
func (r *Registry) publish(action eventschema.EventAction, connector schema.Connector) {
	if r.broadcaster == nil {
		return
	}
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
