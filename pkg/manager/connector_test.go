package manager

import (
	"context"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

// testConnectorURL is a real MCP server used for integration tests.
const testConnectorURL = "https://api.githubcopilot.com/mcp/"

///////////////////////////////////////////////////////////////////////////////
// MOCK CONNECTOR

// mockConnector is an in-process fake that satisfies llm.Connector and the
// prober interface so the manager's CreateConnector path can exercise its
// probe-and-persist logic without hitting an external server.
type mockConnector struct{}

func (mockConnector) Run(context.Context) error                             { return nil }
func (mockConnector) ListTools(context.Context) ([]llm.Tool, error)         { return nil, nil }
func (mockConnector) ListPrompts(context.Context) ([]llm.Prompt, error)     { return nil, nil }
func (mockConnector) ListResources(context.Context) ([]llm.Resource, error) { return nil, nil }
func (mockConnector) Probe(context.Context) (*schema.ConnectorState, error) {
	now := time.Now()
	name := "mock-server"
	version := "0.0.0-test"
	return &schema.ConnectorState{
		ConnectedAt:  &now,
		Name:         &name,
		Version:      &version,
		Capabilities: []schema.Capability{schema.CapabilityTools},
	}, nil
}

// mockConnectorFactory returns a ConnectorFactory that always succeeds with a
// mockConnector, requiring no network access.
func mockConnectorFactory() ConnectorFactory {
	return func(_ context.Context, _ string, _ ...client.ClientOpt) (llm.Connector, error) {
		return mockConnector{}, nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// TEST HELPERS

// newManagerWithFactory creates a Manager that uses a mock connector factory
// so tests can exercise the full CreateConnector path (including probe) without
// any external dependency.
func newManagerWithFactory(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager("test", "0.0.0", WithConnectorFactory(mockConnectorFactory()))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

///////////////////////////////////////////////////////////////////////////////
// CONNECTOR TESTS

// Test CreateConnector probes and stores connector state
func Test_connector_001(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithFactory(t)

	c, err := m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("mcp")})
	assert.NoError(err)
	assert.NotNil(c)
	assert.Equal(testConnectorURL, c.URL)
	assert.True(*c.Enabled)
	assert.Equal("mcp", types.Value(c.Namespace))
	assert.NotNil(c.ConnectedAt)
}

// Test WithConnectorStore rejects nil store
func Test_connector_002(t *testing.T) {
	assert := assert.New(t)

	_, err := NewManager("test", "0.0.0", WithConnectorStore(nil))
	assert.ErrorIs(err, schema.ErrBadParameter)
}

// Test CreateConnector rejects invalid URL before any probe
func Test_connector_003(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	_, err = m.CreateConnector(context.TODO(), "ftp://example.com/sse", schema.ConnectorMeta{})
	assert.Error(err)

	_, err = m.CreateConnector(context.TODO(), "example.com/sse", schema.ConnectorMeta{})
	assert.Error(err)
}

// Test CreateConnector rejects invalid namespace before any probe
func Test_connector_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Namespace: types.Ptr("bad namespace")})
	assert.Error(err)
}

// Test duplicate CreateConnector returns conflict error
func Test_connector_005(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithFactory(t)
	var err error

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: types.Ptr(true)})
	assert.NoError(err)

	// Second registration is rejected at the store level before any probe.
	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: types.Ptr(true)})
	assert.ErrorIs(err, schema.ErrConflict)
}

// Test GetConnector round-trip and not-found
func Test_connector_006(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithFactory(t)
	var err error

	_, err = m.GetConnector(context.TODO(), testConnectorURL)
	assert.ErrorIs(err, schema.ErrNotFound)

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("ns")})
	assert.NoError(err)

	got, err := m.GetConnector(context.TODO(), testConnectorURL)
	assert.NoError(err)
	assert.Equal(testConnectorURL, got.URL)
	assert.Equal("ns", types.Value(got.Namespace))
}

// Test UpdateConnector modifies meta and returns updated connector
func Test_connector_007(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithFactory(t)
	var err error

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: types.Ptr(false), Namespace: types.Ptr("old")})
	assert.NoError(err)

	updated, err := m.UpdateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("new")})
	assert.NoError(err)
	assert.True(*updated.Enabled)
	assert.Equal("new", types.Value(updated.Namespace))
}

// Test UpdateConnector returns not-found for unknown URL
func Test_connector_008(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	_, err = m.UpdateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{})
	assert.ErrorIs(err, schema.ErrNotFound)
}

// Test DeleteConnector removes the connector
func Test_connector_009(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithFactory(t)
	var err error

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: types.Ptr(true)})
	assert.NoError(err)

	assert.NoError(m.DeleteConnector(context.TODO(), testConnectorURL))

	_, err = m.GetConnector(context.TODO(), testConnectorURL)
	assert.ErrorIs(err, schema.ErrNotFound)
}

// Test DeleteConnector returns not-found for unknown URL
func Test_connector_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	err = m.DeleteConnector(context.TODO(), testConnectorURL)
	assert.ErrorIs(err, schema.ErrNotFound)
}

// Test URL canonicalisation strips query params and normalises case
func Test_connector_011(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithFactory(t)

	// Upper-case scheme/host and a spurious query param should both be stripped.
	c, err := m.CreateConnector(context.TODO(), "HTTPS://API.GITHUBCOPILOT.COM/MCP/?token=abc", schema.ConnectorMeta{Enabled: types.Ptr(true)})
	assert.NoError(err)
	assert.Equal(testConnectorURL, c.URL)

	got, err := m.GetConnector(context.TODO(), "HTTPS://API.GITHUBCOPILOT.COM/MCP/?token=abc")
	assert.NoError(err)
	assert.Equal(testConnectorURL, got.URL)
}
