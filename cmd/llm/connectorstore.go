//go:build !client

package main

import (
	"cmp"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	credentials "github.com/mutablelogic/go-llm/auth/credentials"
	mcpregistry "github.com/mutablelogic/go-llm/mcp/registry"
	mcpschema "github.com/mutablelogic/go-llm/mcp/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// connectorStore is a store of the MCP connectors in a file, as a list of
// their definitions, with their serials, in order of their names, under one
// key, so they're kept when the server restarts. How their servers are
// signed in to is kept by the credentials provider. It's safe to use
// concurrently.
type connectorStore struct {
	mu       sync.Mutex
	defaults defaults
}

var _ mcpregistry.Store = (*connectorStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// connectorsKey is the key which the connectors are kept under, and
	// serialsKey the key which the last serial of each name is, so one
	// which was deleted isn't used again, once the server restarts
	connectorsKey = "mcp.connectors"
	serialsKey    = "mcp.serials"

	// credentialsKey is the key which how each connector's server was
	// signed in to, such as its token, was kept under, by its name, before
	// the credentials provider kept it, which is moved to it once
	credentialsKey = "mcp.credentials"

	// connectorsFile is the name of the file which the server keeps the
	// connectors in, in the directory of the command's defaults
	connectorsFile = "connectors.json"

	// credentialsFile is the name of the file which the server keeps
	// credentials in, such as how connectors' servers are signed in to
	credentialsFile = "credentials.json"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newConnectorStore returns a store of the connectors in the defaults
func newConnectorStore(d defaults) (*connectorStore, error) {
	if d == nil {
		return nil, llm.ErrBadParameter.With("missing defaults")
	}
	return &connectorStore{defaults: d}, nil
}

// newServerConnectorStore returns the store of the connectors, in a file of
// its own, connectors.json, in the directory of the command's defaults
func newServerConnectorStore(ctx server.Cmd) (*connectorStore, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	file, err := newJSONFile(filepath.Join(dir, connectorsFile))
	if err != nil {
		return nil, err
	}
	return newConnectorStore(file)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// List returns the connectors in the defaults, in order of their names
func (s *connectorStore) List(context.Context) ([]mcpschema.Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

// Put puts the connector in the defaults, without its status, when its
// serial is zero, and there's no connector with its name, or replaces the
// connector with its name, when its serial is that connector's, and returns
// it, with its new serial, which is one more. Returns ErrBadParameter when
// it has no name, and ErrConflict when its serial isn't the connector's.
func (s *connectorStore) Put(_ context.Context, connector mcpschema.Connector) (mcpschema.Connector, error) {
	if connector.Name == "" {
		return mcpschema.Connector{}, llm.ErrBadParameter.With("missing connector name")
	}
	connector = connector.Definition()
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return mcpschema.Connector{}, err
	}
	var serial uint64
	if i := slices.IndexFunc(list, func(other mcpschema.Connector) bool { return other.Name == connector.Name }); i >= 0 {
		serial = list[i].Serial
		list = slices.Delete(list, i, i+1)
	}
	if connector.Serial != serial {
		return mcpschema.Connector{}, llm.ErrConflict.Withf("connector %q is at serial %d, not %d", connector.Name, serial, connector.Serial)
	}

	// Its serial is one more than the last of its name, which can be one
	// which was deleted, so it isn't used again
	serials, err := s.serials()
	if err != nil {
		return mcpschema.Connector{}, err
	}
	connector.Serial = max(serial, serials[connector.Name]) + 1
	if err := s.write(append(list, connector)); err != nil {
		return mcpschema.Connector{}, err
	}
	return connector, nil
}

// Delete deletes the connector with the name from the defaults, when the
// serial is its serial, or does nothing when there's none. Returns
// ErrConflict when the serial isn't its serial.
func (s *connectorStore) Delete(_ context.Context, name string, serial uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.read()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(connector mcpschema.Connector) bool { return connector.Name == name })
	if i < 0 {
		return nil
	} else if list[i].Serial != serial {
		return llm.ErrConflict.Withf("connector %q is at serial %d, not %d", name, list[i].Serial, serial)
	}

	// Its serial is kept, so it isn't used again for its name
	serials, err := s.serials()
	if err != nil {
		return err
	}
	serials[name] = max(serials[name], serial)
	if err := s.defaults.Set(serialsKey, serials); err != nil {
		return err
	}
	return s.write(slices.Delete(list, i, i+1))
}

// serials returns the last serial of each name of a connector which was
// deleted, or none. The caller holds the lock.
func (s *connectorStore) serials() (map[string]uint64, error) {
	var value any
	if l, ok := s.defaults.(lookup); ok {
		var err error
		if value, err = l.Lookup(serialsKey); err != nil {
			return nil, err
		}
	} else {
		value = s.defaults.Get(serialsKey)
	}
	result := make(map[string]uint64)
	if value == nil {
		return result, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, llm.ErrInternalServerError.Withf("%s: %v", serialsKey, err)
	}
	return result, nil
}

// MoveCredentials moves how each connector's server was signed in to, such
// as its token, which was kept with the connectors, to the credentials
// provider, under its key, so the connectors don't need to be signed in to
// again, and removes them, or does nothing, when there are none
func (s *connectorStore) MoveCredentials(ctx context.Context, provider credentials.Provider) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.credentials()
	if err != nil {
		return err
	} else if len(list) == 0 {
		return nil
	}
	for name, credential := range list {
		data, err := json.Marshal(credential)
		if err != nil {
			return err
		}
		if err := provider.Set(ctx, mcpregistry.CredentialKey(name), string(data)); err != nil {
			return err
		}
	}
	return s.defaults.Set(credentialsKey, nil)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// credentials returns the credentials of the connectors, by their names,
// as they were kept with them, or none. The caller holds the lock.
func (s *connectorStore) credentials() (map[string]mcpschema.Credential, error) {
	var value any
	if l, ok := s.defaults.(lookup); ok {
		var err error
		if value, err = l.Lookup(credentialsKey); err != nil {
			return nil, err
		}
	} else {
		value = s.defaults.Get(credentialsKey)
	}
	result := make(map[string]mcpschema.Credential)
	if value == nil {
		return result, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, llm.ErrInternalServerError.Withf("%s: %v", credentialsKey, err)
	}
	return result, nil
}

// read returns the connectors in the defaults, or none when there are none,
// or the error, when they can't be read, rather than none, so the
// connectors aren't disconnected. The caller holds the lock.
func (s *connectorStore) read() ([]mcpschema.Connector, error) {
	var value any
	if l, ok := s.defaults.(lookup); ok {
		var err error
		if value, err = l.Lookup(connectorsKey); err != nil {
			return nil, err
		}
	} else {
		value = s.defaults.Get(connectorsKey)
	}
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var list []mcpschema.Connector
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, llm.ErrInternalServerError.Withf("%s: %v", connectorsKey, err)
	}
	return list, nil
}

// write sets the connectors in the defaults, in order of their names, or
// removes them, when there are none. The caller holds the lock.
func (s *connectorStore) write(list []mcpschema.Connector) error {
	if len(list) == 0 {
		return s.defaults.Set(connectorsKey, nil)
	}
	slices.SortFunc(list, func(a, b mcpschema.Connector) int { return cmp.Compare(a.Name, b.Name) })
	return s.defaults.Set(connectorsKey, list)
}
