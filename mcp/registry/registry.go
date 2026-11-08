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
	authclient "github.com/mutablelogic/go-llm/auth/httpclient"
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

	// The serial of each connector which was deleted, as it was, so a
	// definition which was read before it was, isn't connected
	removed map[string]uint64

	// The authorizations which are waited for, by their states, and the
	// lock of the connectors' credentials, as their connectors are changed,
	// so a sign-in which finishes as one is changed doesn't keep a
	// credential for it, as it was
	authorizations authorizations
	credentialsMu  sync.Mutex

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
	serial    uint64
	client    *client.Client
	connector schema.Connector
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a registry, with the options, such as its store, with
// WithStore, which keeps its connectors in memory otherwise
func New(opts ...Opt) (*Registry, error) {
	r := &Registry{clients: make(map[string]entry), removed: make(map[string]uint64), namespaces: make(map[string]*namespace.Namespace), events: broadcaster.New()}
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
// It's connected by Run. One without a name is named after its server, as
// it says, once it's connected to, or as the metadata of how it's
// authorized says, when it needs to be, or else the host of its URL, as an
// identifier, with a number, such as github-2, when that's another
// connector's. Returns ErrBadParameter when its name, or its fields, aren't
// valid, and ErrConflict when there's a connector with its name.
func (r *Registry) CreateConnector(ctx context.Context, connector schema.Connector) (*schema.Connector, error) {
	connector = connector.Definition()
	if strings.TrimSpace(connector.Name) == "" {
		return r.createNamed(ctx, connector)
	}
	return r.create(ctx, connector)
}

// create creates the connector, with its name
func (r *Registry) create(ctx context.Context, connector schema.Connector) (*schema.Connector, error) {
	connector.Serial = 0
	if err := connector.Validate(); err != nil {
		return nil, err
	} else if _, _, err := r.clientCredentials(ctx, connector); err != nil {
		return nil, err
	} else if _, err := r.newClient(connector); err != nil {
		return nil, err
	}
	created, err := r.store.Put(ctx, connector)
	if err != nil {
		return nil, err
	}
	r.publish(eventschema.EventCreate, created)
	r.ensureNamespace(created)
	return r.status(created), nil
}

// UpdateConnector replaces the fields of the connector with the name, once
// its server, as it is now, is connected, within its connect timeout, so a URL
// which isn't a server's isn't stored, and returns it, with its new serial,
// and connected. Its server, as it was, is disconnected, once it's
// replaced. One whose URL, and client, are the same, such as when only its
// patterns change, isn't connected first, as its server was already: it
// keeps its session, when it's connected, and has its tools added again,
// with its patterns, or is stored, and connected again, by Run, when it
// isn't, so a server which has only one session at once, or needs to be
// signed in to, isn't connected to twice. Returns ErrNotFound when there's no connector with the name,
// ErrBadParameter when the fields aren't valid, ErrServiceUnavailable when
// its server, as it is now, can't be connected, when it's left as it was,
// and ErrConflict when it was changed as it was updated.
func (r *Registry) UpdateConnector(ctx context.Context, name string, meta schema.ConnectorMeta) (*schema.Connector, error) {
	connector, err := r.get(ctx, name)
	if err != nil {
		return nil, err
	}
	before := connector
	connector.ConnectorMeta = meta
	connector = connector.Definition()
	if err := connector.Validate(); err != nil {
		return nil, err
	} else if _, _, err := r.clientCredentials(ctx, connector); err != nil {
		return nil, err
	}

	// One whose URL, and client, are the same, such as when only its
	// patterns change, isn't connected to first: it keeps its session, when
	// it's connected, as some servers have only one at once, and has its
	// tools added again, or it's stored, and connected again, by Run, as it
	// is now, when it isn't, such as when it's being connected, or needs to
	// be signed in to
	if before.URL == connector.URL && before.ClientIDCredential == connector.ClientIDCredential {
		r.mu.Lock()
		e, exists := r.clients[name]
		r.mu.Unlock()
		if exists && e.serial == connector.Serial && e.client.Connected() {
			return r.update(ctx, connector, e.client)
		}
		updated, err := r.store.Put(ctx, connector)
		if err != nil {
			return nil, err
		}
		r.reconnectOlder(updated)
		return r.status(updated), nil
	}

	// Otherwise connect to its server, as it is now, before it's stored
	c, err := r.newClient(connector)
	if err != nil {
		return nil, err
	}
	connectCtx, cancel := context.WithTimeout(ctx, r.connectTimeout)
	defer cancel()
	needsAuth := false
	if err := c.Connect(connectCtx); errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return nil, llm.ErrServiceUnavailable.Withf("%s didn't reply within %v", connector.URL, r.connectTimeout)
	} else if authclient.AsAuthError(err) != nil {
		// A server which needs to be authorized is one, so it's stored, and
		// said to need to be
		needsAuth = true
	} else if err != nil {
		return nil, err
	}

	// Store it, and replace its client, as it was, with the one which is
	// connected, or disconnect it, when it couldn't be stored. Its
	// credential is deleted, as one, when its server, or client, is another,
	// so its token isn't sent to another server, and a sign-in which
	// finishes as it's updated doesn't keep one
	r.credentialsMu.Lock()
	previous, err := r.get(ctx, name)
	if err != nil {
		r.credentialsMu.Unlock()
		_ = c.Disconnect()
		return nil, err
	}
	updated, err := r.store.Put(ctx, connector)
	if err == nil && (previous.URL != updated.URL || previous.ClientIDCredential != updated.ClientIDCredential) {
		if err := r.credentials.DeleteCredential(ctx, name); err != nil {
			r.log().WarnContext(ctx, "connector's credential not deleted", "connector", name, "error", err.Error())
		}
	}
	r.credentialsMu.Unlock()
	if err != nil {
		_ = c.Disconnect()
		return nil, err
	}
	r.publish(eventschema.EventUpdate, updated)
	if r.replace(updated, c) {
		if server, err := c.Status(); server != nil {
			// It was connected before it was the connector's, so it's said
			// now
			r.connected(updated, c, server)
		} else if needsAuth {
			r.publishAuthRequired(updated.Name, c, err)
		}
	}
	return r.status(updated), nil
}

// reconnectOlder disconnects the connector's client, when it's of an older
// revision than the connector, so Run connects it again, as it is now, but
// not one of the same, or a newer, revision, which Run, or another update,
// may have connected since it was stored, so that session isn't ended
func (r *Registry) reconnectOlder(connector schema.Connector) {
	r.mu.Lock()
	e, exists := r.clients[connector.Name]
	older := exists && e.serial < connector.Serial
	if older {
		delete(r.clients, connector.Name)
	}
	r.mu.Unlock()
	if older {
		_ = r.disconnect(connector.Name, e)
	}
	r.publish(eventschema.EventUpdate, connector)
}

// update stores the connector, whose server the client is connected to, at
// its URL, as it was, and keeps the client, as it is now, and adds its
// tools again, with its patterns
func (r *Registry) update(ctx context.Context, connector schema.Connector, c *client.Client) (*schema.Connector, error) {
	updated, err := r.store.Put(ctx, connector)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	e, exists := r.clients[connector.Name]
	kept := exists && e.client == c && e.serial < updated.Serial
	if kept {
		r.clients[connector.Name] = entry{serial: updated.Serial, client: c, connector: updated.Definition()}
	}
	r.mu.Unlock()
	r.publish(eventschema.EventUpdate, updated)
	if server, _ := c.Status(); kept && server != nil {
		r.connected(updated, c, server)
	}
	return r.status(updated), nil
}

// DeleteConnector deletes the connector with the name, and disconnects its
// server, when it's connected, and returns it, as it was. Returns
// ErrNotFound when there's no connector with the name, and ErrConflict when
// it was changed as it was deleted.
func (r *Registry) DeleteConnector(ctx context.Context, name string) (*schema.Connector, error) {
	// It's deleted, with its credential, as one, so a sign-in which
	// finishes as it's deleted doesn't keep a credential for it
	r.credentialsMu.Lock()
	connector, err := r.get(ctx, name)
	if err != nil {
		r.credentialsMu.Unlock()
		return nil, err
	} else if err := r.store.Delete(ctx, name, connector.Serial); err != nil {
		r.credentialsMu.Unlock()
		return nil, err
	}
	if err := r.credentials.DeleteCredential(ctx, name); err != nil {
		r.log().WarnContext(ctx, "connector's credential not deleted", "connector", name, "error", err.Error())
	}
	r.credentialsMu.Unlock()

	// Its client is disconnected once the lock is released, as its token
	// can be refreshed, and stored, as it's disconnected
	r.forget(name, connector.Serial)
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
//
// A definition which is older than the one the registry has, such as one
// read before it was updated, or deleted, isn't the connector's, as it is,
// so it has no client, and ErrConflict is returned, or ErrNotFound, when it
// was deleted. The client it replaces, when it's connected, is said to be
// disconnected, and its tools are cleared, until the new one is connected.
func (r *Registry) client(connector schema.Connector) (*client.Client, error) {
	r.mu.Lock()
	if removed, exists := r.removed[connector.Name]; exists && connector.Serial <= removed {
		r.mu.Unlock()
		return nil, llm.ErrNotFound.Withf("connector %q was deleted", connector.Name)
	}
	e, exists := r.clients[connector.Name]
	if exists && e.serial == connector.Serial {
		r.mu.Unlock()
		return e.client, nil
	} else if exists && e.serial > connector.Serial {
		r.mu.Unlock()
		return nil, llm.ErrConflict.Withf("connector %q was changed", connector.Name)
	}
	c, err := r.newClient(connector)
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	r.clients[connector.Name] = entry{serial: connector.Serial, client: c, connector: connector.Definition()}
	r.mu.Unlock()
	if exists {
		_ = r.disconnect(connector.Name, e)
	}
	return c, nil
}

// replace makes the client the connector's, as it is, and disconnects the
// one it replaces, which isn't said, as the client which replaces it is
// connected, and returns true, or disconnects the client, and returns false,
// when the connector was changed, or deleted, since, so it isn't
func (r *Registry) replace(connector schema.Connector, c *client.Client) bool {
	r.mu.Lock()
	e, exists := r.clients[connector.Name]
	removed, deleted := r.removed[connector.Name]
	if (deleted && connector.Serial <= removed) || (exists && e.serial > connector.Serial) {
		r.mu.Unlock()
		_ = c.Disconnect()
		return false
	}
	r.clients[connector.Name] = entry{serial: connector.Serial, client: c, connector: connector.Definition()}
	r.mu.Unlock()
	if exists && e.client != c {
		// Its tools are cleared, unless they're another client's, as the
		// client's change isn't said, once it's been replaced
		_ = e.client.Disconnect()
		r.disconnectNamespace(connector.Name, e.client)
	}
	return true
}

// disconnect disconnects the client of the connector with the name, which
// is no longer the connector's, and says it was, when it was connected
func (r *Registry) disconnect(name string, e entry) error {
	connected := e.client.Connected()
	err := e.client.Disconnect()
	if connected {
		r.publishConnection(name, e.serial, nil, nil)
		r.disconnectNamespace(name, e.client)
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
// forgets it, and its namespace, as the connector was deleted, at the
// serial, or another process deleted it, so a definition which is older
// isn't connected
func (r *Registry) forget(name string, serial uint64) {
	r.mu.Lock()
	e, exists := r.clients[name]
	delete(r.clients, name)
	if exists {
		serial = max(serial, e.serial)
	}
	r.removed[name] = max(r.removed[name], serial)
	r.mu.Unlock()
	if exists {
		_ = r.disconnect(name, e)
	}
	r.removeNamespace(name)
}

// status returns the connector, with the namespace of its tools, which it
// has whether it's connected, or not, and whether its server is connected,
// and which it is, or why it couldn't be connected, by its client, when it
// has one, for the connector as it is
func (r *Registry) status(connector schema.Connector) *schema.Connector {
	result := connector.Definition()
	result.Namespace = r.namespaceOf(connector.Name)
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
		if e.client.Authorized() {
			result.Auth = schema.AuthAuthorized
		}
	}
	if err != nil {
		result.Error = err.Error()
		if authclient.AsAuthError(err) != nil {
			result.Auth = schema.AuthRequired
		}
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

// connected publishes that the connector's server is connected, with the
// client, which is the connector's, and lists its tools in its namespace
func (r *Registry) connected(connector schema.Connector, c *client.Client, server *sdkmcp.Implementation) {
	r.publishConnection(connector.Name, connector.Serial, server, nil)
	r.connectNamespace(c, server, connector)
}

// publishAuthRequired publishes that the connector's server needs to be
// authorized, as the client couldn't be connected, with why, as an update
// of the connector, while it's the connector's client
func (r *Registry) publishAuthRequired(name string, c *client.Client, err error) {
	if r.broadcaster == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, exists := r.clients[name]
	if !exists || e.client != c {
		return
	}
	data := schema.ConnectorEvent{Serial: e.serial, Auth: schema.AuthRequired}
	if err != nil {
		data.Error = err.Error()
	}
	event := eventschema.Event{Action: eventschema.EventUpdate, Schema: schema.Name, Table: schema.ConnectorTable, Row: name}
	if raw, err := json.Marshal(data); err == nil {
		event.Data = raw
	}
	r.broadcaster.Publish(event)
}

// publishConnection publishes that the connector's server was connected,
// with which server it is, as it said, or disconnected, with why, when it
// wasn't disconnected, with the broadcaster the registry was given, when it
// was, as a connector which is created, or deleted. It never waits, so it
// can be called with the lock held.
func (r *Registry) publishConnection(name string, serial uint64, server *sdkmcp.Implementation, err error) {
	if r.broadcaster == nil {
		return
	}
	connector := schema.Connector{Name: name, Serial: serial}
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
		// Whether it's still the connector's client, and the connection's
		// said, are as one, so it isn't said once it's been replaced
		r.mu.Lock()
		e, current := r.clients[connector.Name]
		current = current && e.client == c
		if current {
			r.publishConnection(connector.Name, e.serial, server, err)
		}
		r.mu.Unlock()
		switch {
		case !current:
			return
		case server != nil:
			r.connectNamespace(c, server, e.connector)
		default:
			r.disconnectNamespace(connector.Name, c)
		}
	}), client.WithToolsChanged(func() {
		if r.current(connector.Name, c) {
			r.refreshNamespace(connector.Name)
		}
	}))
	if tokens := r.tokenSource(connector); tokens != nil {
		opts = append(opts, client.WithTokenSource(tokens))
	}
	var err error
	c, err = client.New(connector.URL, opts...)
	return c, err
}
