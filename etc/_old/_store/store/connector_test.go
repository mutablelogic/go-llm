package store_test

import (
	"context"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

func connectorInsert(rawURL string, meta schema.ConnectorMeta) schema.ConnectorInsert {
	return schema.ConnectorInsert{URL: rawURL, ConnectorMeta: meta}
}

// connectorStoreTests defines shared behavioural tests for any
// ConnectorStore implementation.
var connectorStoreTests = []struct {
	Name string
	Fn   func(t *testing.T, s schema.ConnectorStore)
}{{
	Name: "CreateAndGet",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		c, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("mcp")}))
		a.NoError(err)
		a.NotNil(c)
		a.Equal("https://example.com/sse", c.URL)
		a.True(types.Value(c.Enabled))
		a.Equal("mcp", types.Value(c.Namespace))
		a.Nil(c.Meta)
		a.False(c.CreatedAt.IsZero())
		got, err := s.GetConnector(ctx, "https://example.com/sse")
		a.NoError(err)
		a.Equal(c.URL, got.URL)
		a.Equal(types.Value(c.Namespace), types.Value(got.Namespace))
	},
}, {
	Name: "CreateDuplicateConflict",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
		a.NoError(err)
		_, err = s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
		a.Error(err)
	},
}, {
	Name: "CreateInvalidURL",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		// Missing scheme
		_, err := s.CreateConnector(ctx, connectorInsert("example.com/sse", schema.ConnectorMeta{}))
		a.Error(err)
		// Unsupported scheme
		_, err = s.CreateConnector(ctx, connectorInsert("ftp://example.com/sse", schema.ConnectorMeta{}))
		a.Error(err)
		// Empty URL
		_, err = s.CreateConnector(ctx, connectorInsert("", schema.ConnectorMeta{}))
		a.Error(err)
		// Invalid port
		_, err = s.CreateConnector(ctx, connectorInsert("https://example.com:99999/sse", schema.ConnectorMeta{}))
		a.Error(err)
	},
}, {
	Name: "CreateInvalidNamespace",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		// Starts with a digit
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("1bad")}))
		a.Error(err)
		// Contains spaces
		_, err = s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("has spaces")}))
		a.Error(err)
	},
}, {
	Name: "CreateURLCanonicalised",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		// Uppercase scheme and host are lowercased; query and fragment are stripped.
		c, err := s.CreateConnector(ctx, connectorInsert("HTTPS://Example.COM/sse?token=abc#frag", schema.ConnectorMeta{}))
		a.NoError(err)
		a.Equal("https://example.com/sse", c.URL)
		// Lookup with the original (non-canonical) URL must still work.
		got, err := s.GetConnector(ctx, "HTTPS://Example.COM/sse?token=abc#frag")
		a.NoError(err)
		a.Equal("https://example.com/sse", got.URL)
	},
}, {
	Name: "GetNotFound",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		_, err := s.GetConnector(context.Background(), "https://example.com/sse")
		a.Error(err)
	},
}, {
	Name: "UpdateMeta",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(false), Namespace: types.Ptr("old")}))
		a.NoError(err)
		updated, err := s.UpdateConnector(ctx, "https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("new")})
		a.NoError(err)
		a.True(types.Value(updated.Enabled))
		a.Equal("new", types.Value(updated.Namespace))
	},
}, {
	Name: "CreateAndUpdateMetaObject",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		created, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{
			Enabled:   types.Ptr(true),
			Namespace: types.Ptr("mcp"),
			Meta:      schema.ProviderMetaMap{"env": "dev", "retries": float64(3)},
		}))
		a.NoError(err)
		a.Equal(schema.ProviderMetaMap{"env": "dev", "retries": float64(3)}, created.Meta)

		updated, err := s.UpdateConnector(ctx, "https://example.com/sse", schema.ConnectorMeta{
			Meta: schema.ProviderMetaMap{"env": "prod", "labels": map[string]any{"team": "platform"}},
		})
		a.NoError(err)
		a.Equal(schema.ProviderMetaMap{"env": "prod", "labels": map[string]any{"team": "platform"}}, updated.Meta)
	},
}, {
	Name: "UpdateMetaPartialEnabled",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(false), Namespace: types.Ptr("keep"), Meta: schema.ProviderMetaMap{"env": "dev"}}))
		a.NoError(err)
		// Update only Enabled; Namespace and Meta are nil and must be preserved.
		updated, err := s.UpdateConnector(ctx, "https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)})
		a.NoError(err)
		a.True(types.Value(updated.Enabled))
		a.Equal("keep", types.Value(updated.Namespace))
		a.Equal(schema.ProviderMetaMap{"env": "dev"}, updated.Meta)
	},
}, {
	Name: "UpdateMetaPartialNamespace",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("old"), Meta: schema.ProviderMetaMap{"env": "dev"}}))
		a.NoError(err)
		// Update only Namespace; Enabled and Meta are nil and must be preserved.
		updated, err := s.UpdateConnector(ctx, "https://example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("renamed")})
		a.NoError(err)
		a.True(types.Value(updated.Enabled))
		a.Equal("renamed", types.Value(updated.Namespace))
		a.Equal(schema.ProviderMetaMap{"env": "dev"}, updated.Meta)
	},
}, {
	Name: "UpdateMetaClearObject",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Meta: schema.ProviderMetaMap{"env": "dev"}}))
		a.NoError(err)

		updated, err := s.UpdateConnector(ctx, "https://example.com/sse", schema.ConnectorMeta{Meta: schema.ProviderMetaMap{}})
		a.NoError(err)
		a.NotNil(updated.Meta)
		a.Empty(updated.Meta)
	},
}, {
	Name: "UpdateNotFound",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		_, err := s.UpdateConnector(context.Background(), "https://example.com/sse", schema.ConnectorMeta{})
		a.Error(err)
	},
}, {
	Name: "UpdateInvalidNamespace",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{}))
		a.NoError(err)
		_, err = s.UpdateConnector(ctx, "https://example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("bad namespace")})
		a.Error(err)
	},
}, {
	Name: "UpdateDuplicateNamespace",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://a.example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("ns1")}))
		a.NoError(err)
		_, err = s.CreateConnector(ctx, connectorInsert("https://b.example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("ns2")}))
		a.NoError(err)
		// Trying to rename b's namespace to an already-used one must fail.
		_, err = s.UpdateConnector(ctx, "https://b.example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("ns1")})
		a.Error(err)
		// Updating a connector to its own namespace must succeed (no conflict with self).
		_, err = s.UpdateConnector(ctx, "https://a.example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("ns1")})
		a.NoError(err)
	},
}, {
	Name: "Delete",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
		a.NoError(err)
		a.NoError(s.DeleteConnector(ctx, "https://example.com/sse"))
		_, err = s.GetConnector(ctx, "https://example.com/sse")
		a.Error(err)
	},
}, {
	Name: "DeleteNotFound",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		a.Error(s.DeleteConnector(context.Background(), "https://example.com/sse"))
	},
}, {
	Name: "ListEmpty",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		resp, err := s.ListConnectors(context.Background(), schema.ConnectorListRequest{})
		a.NoError(err)
		a.Equal(uint(0), resp.Count)
		a.Empty(resp.Body)
	},
}, {
	Name: "ListAll",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://a.example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("ns1")}))
		a.NoError(err)
		_, err = s.CreateConnector(ctx, connectorInsert("https://b.example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(false), Namespace: types.Ptr("ns2")}))
		a.NoError(err)
		resp, err := s.ListConnectors(ctx, schema.ConnectorListRequest{})
		a.NoError(err)
		a.Equal(uint(2), resp.Count)
		a.Len(resp.Body, 2)
	},
}, {
	Name: "CreateDuplicateNamespace",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://a.example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("ns1")}))
		a.NoError(err)
		_, err = s.CreateConnector(ctx, connectorInsert("https://b.example.com/sse", schema.ConnectorMeta{Namespace: types.Ptr("ns1")}))
		a.Error(err)
	},
}, {
	Name: "ListFilterNamespace",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		// Each namespace is unique; create three connectors with distinct namespaces.
		_, err := s.CreateConnector(ctx, connectorInsert("https://a.example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("ns1")}))
		a.NoError(err)
		_, err = s.CreateConnector(ctx, connectorInsert("https://b.example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("ns2")}))
		a.NoError(err)
		_, err = s.CreateConnector(ctx, connectorInsert("https://c.example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("ns3")}))
		a.NoError(err)
		// Filter by ns1 should return exactly one result.
		resp, err := s.ListConnectors(ctx, schema.ConnectorListRequest{Namespace: "ns1"})
		a.NoError(err)
		a.Equal(uint(1), resp.Count)
		a.Len(resp.Body, 1)
		a.Equal("ns1", types.Value(resp.Body[0].Namespace))
		// Filter by ns2 should return exactly one result.
		resp, err = s.ListConnectors(ctx, schema.ConnectorListRequest{Namespace: "ns2"})
		a.NoError(err)
		a.Equal(uint(1), resp.Count)
		a.Len(resp.Body, 1)
		a.Equal("ns2", types.Value(resp.Body[0].Namespace))
	},
}, {
	Name: "ListFilterEnabled",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://a.example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
		a.NoError(err)
		_, err = s.CreateConnector(ctx, connectorInsert("https://b.example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(false)}))
		a.NoError(err)
		_, err = s.CreateConnector(ctx, connectorInsert("https://c.example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
		a.NoError(err)
		resp, err := s.ListConnectors(ctx, schema.ConnectorListRequest{Enabled: types.Ptr(true)})
		a.NoError(err)
		a.Equal(uint(2), resp.Count)
		a.Len(resp.Body, 2)
		for _, c := range resp.Body {
			a.True(types.Value(c.Enabled))
		}
		resp, err = s.ListConnectors(ctx, schema.ConnectorListRequest{Enabled: types.Ptr(false)})
		a.NoError(err)
		a.Equal(uint(1), resp.Count)
		a.Len(resp.Body, 1)
		a.False(types.Value(resp.Body[0].Enabled))
	},
}, {
	Name: "ListPaginationLimit",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		for i := range 5 {
			_, err := s.CreateConnector(ctx, connectorInsert("https://"+string(rune('a'+i))+".example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
			a.NoError(err)
		}
		limit := uint(2)
		resp, err := s.ListConnectors(ctx, schema.ConnectorListRequest{Limit: &limit})
		a.NoError(err)
		a.Equal(uint(5), resp.Count)
		a.Len(resp.Body, 2)
	},
}, {
	Name: "ListPaginationOffset",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		for i := range 5 {
			_, err := s.CreateConnector(ctx, connectorInsert("https://"+string(rune('a'+i))+".example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
			a.NoError(err)
		}
		resp, err := s.ListConnectors(ctx, schema.ConnectorListRequest{Offset: 3})
		a.NoError(err)
		a.Equal(uint(5), resp.Count)
		a.Len(resp.Body, 2)
		// Offset beyond total returns empty body.
		resp, err = s.ListConnectors(ctx, schema.ConnectorListRequest{Offset: 10})
		a.NoError(err)
		a.Equal(uint(5), resp.Count)
		a.Empty(resp.Body)
	},
}, {
	Name: "UpdateConnectorState",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
		a.NoError(err)
		name := "my-server"
		version := "1.2.3"
		updated, err := s.UpdateConnectorState(ctx, "https://example.com/sse", schema.ConnectorState{
			Name:    &name,
			Version: &version,
		})
		a.NoError(err)
		a.Equal(name, *updated.Name)
		a.Equal(version, *updated.Version)
		// Fields not set must remain nil.
		a.Nil(updated.Title)
		a.Nil(updated.Description)
		a.Nil(updated.ConnectedAt)
	},
}, {
	Name: "UpdateConnectorStateNotFound",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		name := "x"
		_, err := s.UpdateConnectorState(context.Background(), "https://example.com/sse", schema.ConnectorState{Name: &name})
		a.Error(err)
	},
}, {
	Name: "UpdateConnectorStateCapabilities",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true)}))
		a.NoError(err)
		caps := []schema.ConnectorCapability{schema.CapabilityTools, schema.CapabilityResources}
		updated, err := s.UpdateConnectorState(ctx, "https://example.com/sse", schema.ConnectorState{Capabilities: caps})
		a.NoError(err)
		a.Equal(caps, updated.Capabilities)
		// Replace capabilities with a different set.
		caps2 := []schema.ConnectorCapability{schema.CapabilityPrompts}
		updated, err = s.UpdateConnectorState(ctx, "https://example.com/sse", schema.ConnectorState{Capabilities: caps2})
		a.NoError(err)
		a.Equal(caps2, updated.Capabilities)
	},
}, {
	Name: "UpdateConnectorStatePreservesMeta",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{Enabled: types.Ptr(true), Namespace: types.Ptr("ns")}))
		a.NoError(err)
		name := "srv"
		updated, err := s.UpdateConnectorState(ctx, "https://example.com/sse", schema.ConnectorState{Name: &name})
		a.NoError(err)
		// Meta fields must be untouched.
		a.True(types.Value(updated.Enabled))
		a.Equal("ns", types.Value(updated.Namespace))
		a.Nil(updated.Meta)
	},
}, {
	Name: "UpdateConnectorStatePreservesUserMeta",
	Fn: func(t *testing.T, s schema.ConnectorStore) {
		a := assert.New(t)
		ctx := context.Background()
		_, err := s.CreateConnector(ctx, connectorInsert("https://example.com/sse", schema.ConnectorMeta{
			Enabled:   types.Ptr(true),
			Namespace: types.Ptr("ns"),
			Meta:      schema.ProviderMetaMap{"env": "dev"},
		}))
		a.NoError(err)
		name := "srv"
		updated, err := s.UpdateConnectorState(ctx, "https://example.com/sse", schema.ConnectorState{Name: &name})
		a.NoError(err)
		a.Equal(schema.ProviderMetaMap{"env": "dev"}, updated.Meta)
	},
}}

// runConnectorStoreTests runs every shared behavioural test against a
// ConnectorStore implementation. The factory is called once per subtest
// so each gets a clean, independent store.
func runConnectorStoreTests(t *testing.T, factory func() schema.ConnectorStore) {
	t.Helper()
	for _, tt := range connectorStoreTests {
		t.Run(tt.Name, func(t *testing.T) {
			tt.Fn(t, factory())
		})
	}
}
