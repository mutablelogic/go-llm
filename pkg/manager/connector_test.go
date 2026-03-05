package manager

import (
	"context"
	"os"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
	oauth2 "golang.org/x/oauth2"
)

// testConnectorURL is a real MCP server used for integration tests.
const testConnectorURL = "https://api.githubcopilot.com/mcp/"

// newManagerWithAuth creates a Manager preloaded with the GITHUB_TOKEN bearer
// credential for testConnectorURL.  The test is skipped if GITHUB_TOKEN is not set.
func newManagerWithAuth(t *testing.T) *Manager {
	t.Helper()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set")
	}
	credStore, err := store.NewMemoryCredentialStore("test-passphrase-for-manager")
	if err != nil {
		t.Fatalf("NewMemoryCredentialStore: %v", err)
	}
	if err := credStore.SetCredential(context.Background(), testConnectorURL, schema.OAuthCredentials{
		Token: &oauth2.Token{AccessToken: token},
	}); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	m, err := NewManager("test", "0.0.0", WithCredentialStore(credStore))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

///////////////////////////////////////////////////////////////////////////////
// CONNECTOR TESTS

// Test CreateConnector probes the real MCP server and stores its state
func Test_connector_001(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithAuth(t)

	c, err := m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: true, Namespace: "mcp"})
	assert.NoError(err)
	assert.NotNil(c)
	assert.Equal(testConnectorURL, c.URL)
	assert.True(c.Enabled)
	assert.Equal("mcp", c.Namespace)
	assert.NotNil(c.ConnectedAt)
}

// Test WithConnectorStore rejects nil store
func Test_connector_002(t *testing.T) {
	assert := assert.New(t)

	_, err := NewManager("test", "0.0.0", WithConnectorStore(nil))
	assert.ErrorIs(err, llm.ErrBadParameter)
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

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Namespace: "bad namespace"})
	assert.Error(err)
}

// Test duplicate CreateConnector returns conflict error
func Test_connector_005(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithAuth(t)
	var err error

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: true})
	assert.NoError(err)

	// Second registration is rejected at the store level before any probe.
	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: true})
	assert.ErrorIs(err, llm.ErrConflict)
}

// Test GetConnector round-trip and not-found
func Test_connector_006(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithAuth(t)
	var err error

	_, err = m.GetConnector(context.TODO(), testConnectorURL)
	assert.ErrorIs(err, llm.ErrNotFound)

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: true, Namespace: "ns"})
	assert.NoError(err)

	got, err := m.GetConnector(context.TODO(), testConnectorURL)
	assert.NoError(err)
	assert.Equal(testConnectorURL, got.URL)
	assert.Equal("ns", got.Namespace)
}

// Test UpdateConnector modifies meta and returns updated connector
func Test_connector_007(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithAuth(t)
	var err error

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: false, Namespace: "old"})
	assert.NoError(err)

	updated, err := m.UpdateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: true, Namespace: "new"})
	assert.NoError(err)
	assert.True(updated.Enabled)
	assert.Equal("new", updated.Namespace)
}

// Test UpdateConnector returns not-found for unknown URL
func Test_connector_008(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	_, err = m.UpdateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test DeleteConnector removes the connector
func Test_connector_009(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithAuth(t)
	var err error

	_, err = m.CreateConnector(context.TODO(), testConnectorURL, schema.ConnectorMeta{Enabled: true})
	assert.NoError(err)

	assert.NoError(m.DeleteConnector(context.TODO(), testConnectorURL))

	_, err = m.GetConnector(context.TODO(), testConnectorURL)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test DeleteConnector returns not-found for unknown URL
func Test_connector_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	err = m.DeleteConnector(context.TODO(), testConnectorURL)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test URL canonicalisation strips query params and normalises case
func Test_connector_011(t *testing.T) {
	assert := assert.New(t)

	m := newManagerWithAuth(t)

	// Upper-case scheme/host and a spurious query param should both be stripped.
	c, err := m.CreateConnector(context.TODO(), "HTTPS://API.GITHUBCOPILOT.COM/MCP/?token=abc", schema.ConnectorMeta{})
	assert.NoError(err)
	assert.Equal(testConnectorURL, c.URL)

	got, err := m.GetConnector(context.TODO(), "HTTPS://API.GITHUBCOPILOT.COM/MCP/?token=abc")
	assert.NoError(err)
	assert.Equal(testConnectorURL, got.URL)
}
