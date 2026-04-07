package manager

import (
	"context"
	"errors"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func TestCreateConnector(t *testing.T) {
	conn, m := newIntegrationManager(t)

	if err := m.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	enabled := false
	namespace := "mcp"
	url := llmtest.ConnectorURL(t, "create-connector") + "?token=abc#frag"
	created, _, _, err := m.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: url,
		ConnectorMeta: schema.ConnectorMeta{
			Enabled:   &enabled,
			Namespace: &namespace,
			Groups:    conn.Config.Groups,
			Meta:      schema.ProviderMetaMap{"env": "dev"},
		},
	}, llmtest.AdminUser(conn))
	if err != nil {
		t.Fatal(err)
	}

	if created.URL != newConnectorTestURLCanonical(url) {
		t.Fatalf("unexpected URL: %q", created.URL)
	}
	if types.Value(created.Enabled) {
		t.Fatal("expected connector to be disabled")
	}
	if types.Value(created.Namespace) != "mcp" {
		t.Fatalf("unexpected namespace: %q", types.Value(created.Namespace))
	}
	if created.Meta["env"] != "dev" {
		t.Fatalf("unexpected meta: %v", created.Meta)
	}
	if len(created.Groups) != len(conn.Config.Groups) || created.Groups[0] != conn.Config.Groups[0] {
		t.Fatalf("unexpected groups: %v", created.Groups)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
	if created.ModifiedAt != nil {
		t.Fatalf("expected modified_at to be nil, got %v", *created.ModifiedAt)
	}
}

func TestCreateConnectorRollsBackGroupFailure(t *testing.T) {
	_, m := newIntegrationManager(t)
	url := llmtest.ConnectorURL(t, "rollback-group-failure")

	if err := m.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := m.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: url,
		ConnectorMeta: schema.ConnectorMeta{
			Groups: []string{"missing-group"},
		},
	}, nil)
	if err == nil {
		t.Fatal("expected create connector to fail for missing group")
	}

	var connector schema.Connector
	if err := m.PoolConn.Get(context.Background(), &connector, schema.ConnectorURLSelector(url)); !errors.Is(pg.NormalizeError(err), pg.ErrNotFound) {
		t.Fatalf("expected connector row to be rolled back, got err=%v connector=%v", err, connector)
	}
}

func TestDeleteConnector(t *testing.T) {
	conn, m := newIntegrationManager(t)
	url := llmtest.ConnectorURL(t, "delete-connector")

	if err := m.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := m.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: url,
		ConnectorMeta: schema.ConnectorMeta{
			Groups: conn.Config.Groups,
		},
	}, llmtest.AdminUser(conn))
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := m.DeleteConnector(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.URL != url {
		t.Fatalf("unexpected deleted URL: %q", deleted.URL)
	}

	var connector schema.Connector
	if err := m.PoolConn.Get(context.Background(), &connector, schema.ConnectorURLSelector(url)); !errors.Is(pg.NormalizeError(err), pg.ErrNotFound) {
		t.Fatalf("expected connector to be deleted, got err=%v connector=%v", err, connector)
	}
}

func TestDeleteConnectorNotFound(t *testing.T) {
	_, m := newIntegrationManager(t)

	if _, err := m.DeleteConnector(context.Background(), "https://example.com/sse"); !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestGetConnector(t *testing.T) {
	conn, m := newIntegrationManager(t)

	if err := m.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	publicURL := llmtest.ConnectorURL(t, "public-connector")
	privateURL := llmtest.ConnectorURL(t, "private-connector")
	if _, _, _, err := m.CreateConnector(context.Background(), schema.ConnectorInsert{URL: publicURL}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := m.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL:           privateURL,
		ConnectorMeta: schema.ConnectorMeta{Groups: conn.Config.Groups},
	}, llmtest.AdminUser(conn)); err != nil {
		t.Fatal(err)
	}

	privateAll, err := m.GetConnector(context.Background(), privateURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if privateAll.URL != privateURL {
		t.Fatalf("expected connector URL %q, got %q", privateURL, privateAll.URL)
	}

	privateAdmin, err := m.GetConnector(context.Background(), privateURL, llmtest.AdminUser(conn))
	if err != nil {
		t.Fatal(err)
	}
	if privateAdmin.URL != privateURL {
		t.Fatalf("expected connector URL %q, got %q", privateURL, privateAdmin.URL)
	}

	publicUngrouped, err := m.GetConnector(context.Background(), publicURL, llmtest.User(conn))
	if err != nil {
		t.Fatal(err)
	}
	if publicUngrouped.URL != publicURL {
		t.Fatalf("expected connector URL %q, got %q", publicURL, publicUngrouped.URL)
	}

	if _, err := m.GetConnector(context.Background(), privateURL, llmtest.User(conn)); !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected not found for inaccessible connector, got %v", err)
	}
}

func TestUpdateConnector(t *testing.T) {
	conn, m := newIntegrationManager(t)
	url := llmtest.ConnectorURL(t, "update-connector")

	if err := m.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	enabled := false
	oldNamespace := "mcp"
	created, _, _, err := m.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL: url,
		ConnectorMeta: schema.ConnectorMeta{
			Enabled:   &enabled,
			Namespace: &oldNamespace,
			Groups:    conn.Config.Groups,
			Meta:      schema.ProviderMetaMap{"env": "dev"},
		},
	}, llmtest.AdminUser(conn))
	if err != nil {
		t.Fatal(err)
	}
	if created.ModifiedAt != nil {
		t.Fatalf("expected modified_at to be nil, got %v", *created.ModifiedAt)
	}

	newEnabled := true
	newNamespace := "renamed"
	updated, err := m.UpdateConnector(context.Background(), url, schema.ConnectorMeta{
		Enabled:   &newEnabled,
		Namespace: &newNamespace,
		Groups:    []string{},
		Meta:      schema.ProviderMetaMap{"env": "prod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !types.Value(updated.Enabled) {
		t.Fatal("expected connector to be enabled after update")
	}
	if types.Value(updated.Namespace) != "renamed" {
		t.Fatalf("expected namespace %q, got %q", "renamed", types.Value(updated.Namespace))
	}
	if updated.Meta["env"] != "prod" {
		t.Fatalf("unexpected meta: %v", updated.Meta)
	}
	if len(updated.Groups) != 0 {
		t.Fatalf("expected groups to be cleared, got %v", updated.Groups)
	}
	if updated.ModifiedAt == nil {
		t.Fatal("expected modified_at to be set")
	}
}

func TestListConnectors(t *testing.T) {
	conn, m := newIntegrationManager(t)

	if err := m.Exec(context.Background(), `TRUNCATE llm.connector CASCADE`); err != nil {
		t.Fatal(err)
	}

	publicURL := llmtest.ConnectorURL(t, "list-public-connector")
	privateURL := llmtest.ConnectorURL(t, "list-private-connector")
	if _, _, _, err := m.CreateConnector(context.Background(), schema.ConnectorInsert{URL: publicURL}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := m.CreateConnector(context.Background(), schema.ConnectorInsert{
		URL:           privateURL,
		ConnectorMeta: schema.ConnectorMeta{Groups: conn.Config.Groups},
	}, llmtest.AdminUser(conn)); err != nil {
		t.Fatal(err)
	}

	all, err := m.ListConnectors(context.Background(), schema.ConnectorListRequest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if all.Count != 2 || len(all.Body) != 2 {
		t.Fatalf("expected 2 connectors, got count=%d len=%d", all.Count, len(all.Body))
	}

	admins, err := m.ListConnectors(context.Background(), schema.ConnectorListRequest{}, llmtest.AdminUser(conn))
	if err != nil {
		t.Fatal(err)
	}
	if admins.Count != 2 || len(admins.Body) != 2 {
		t.Fatalf("expected admin user to see 2 connectors, got count=%d len=%d", admins.Count, len(admins.Body))
	}

	ungroupedUser, err := m.ListConnectors(context.Background(), schema.ConnectorListRequest{}, llmtest.User(conn))
	if err != nil {
		t.Fatal(err)
	}
	if ungroupedUser.Count != 1 || len(ungroupedUser.Body) != 1 {
		t.Fatalf("expected ungrouped user to see 1 connector, got count=%d len=%d", ungroupedUser.Count, len(ungroupedUser.Body))
	}
	if ungroupedUser.Body[0].URL != publicURL {
		t.Fatalf("expected public connector %q, got %q", publicURL, ungroupedUser.Body[0].URL)
	}
}

func newConnectorTestURLCanonical(raw string) string {
	url, err := schema.CanonicalURL(raw)
	if err != nil {
		panic(err)
	}
	return url
}
