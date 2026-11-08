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
	mcpregistry "github.com/mutablelogic/go-llm/mcp/registry"
	mcpschema "github.com/mutablelogic/go-llm/mcp/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// connectorStore is a store of the MCP connectors in a file, as a list of
// their definitions, with their serials, in order of their names, under one
// key, so they're kept when the server restarts, and of how their servers
// are authorized, such as their tokens, under another. It's safe to use
// concurrently.
type connectorStore struct {
	mu       sync.Mutex
	defaults defaults
}

var _ mcpregistry.Store = (*connectorStore)(nil)
var _ mcpregistry.Credentials = (*connectorStore)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// connectorsKey is the key which the connectors are kept under, and
	// serialsKey the key which the last serial of each name is, so one
	// which was deleted isn't used again, once the server restarts
	connectorsKey = "mcp.connectors"
	serialsKey    = "mcp.serials"

	// credentialsKey is the key which how each connector's server is
	// authorized, such as its token, is kept under, by its name
	credentialsKey = "mcp.credentials"

	// connectorsFile is the name of the file which the server keeps the
	// connectors in, in the directory of the command's defaults
	connectorsFile = "connectors.json"
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

// GetCredential returns how the connector's server is authorized, or
// ErrNotFound when it has no credential
func (s *connectorStore) GetCredential(_ context.Context, name string) (*mcpschema.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	credentials, err := s.credentials()
	if err != nil {
		return nil, err
	}
	credential, exists := credentials[name]
	if !exists {
		return nil, llm.ErrNotFound.Withf("connector %q has no credential", name)
	}
	return &credential, nil
}

// PutCredential sets how the connector's server is authorized
func (s *connectorStore) PutCredential(_ context.Context, name string, credential mcpschema.Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	credentials, err := s.credentials()
	if err != nil {
		return err
	}
	credentials[name] = credential
	return s.defaults.Set(credentialsKey, credentials)
}

// DeleteCredential deletes how the connector's server is authorized, or
// does nothing when it has no credential
func (s *connectorStore) DeleteCredential(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	credentials, err := s.credentials()
	if err != nil {
		return err
	} else if _, exists := credentials[name]; !exists {
		return nil
	}
	delete(credentials, name)
	if len(credentials) == 0 {
		return s.defaults.Set(credentialsKey, nil)
	}
	return s.defaults.Set(credentialsKey, credentials)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// credentials returns the credentials of the connectors, by their names,
// or none. The caller holds the lock.
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
