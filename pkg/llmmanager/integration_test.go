package manager

import (
	"context"
	"errors"
	"net"
	"testing"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
)

var integrationConn llmtest.Conn

func TestMain(m *testing.M) {
	llmtest.Main(m, &integrationConn, llmtest.ProviderConfig{
		Name:     "restricted-ollama",
		Provider: schema.Ollama,
		Groups:   []string{"admins"},
	})
}

func newIntegrationManager(t *testing.T) (*llmtest.Conn, *Manager) {
	t.Helper()

	conn := integrationConn.Begin(t)
	t.Cleanup(conn.Close)

	ctx := context.Background()
	m, err := New(ctx, conn, WithPassphrase(1, "test1234"))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Exec(ctx, `TRUNCATE llm.provider CASCADE`); err != nil {
		t.Fatal(err)
	}

	return conn, m
}

func createIntegrationProvider(t *testing.T, m *Manager, insert schema.ProviderInsert) *schema.Provider {
	t.Helper()

	provider, err := m.CreateProvider(context.Background(), insert)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.SyncProviders(context.Background()); err != nil {
		t.Fatal(err)
	}

	return provider
}

func integrationAdminUser(conn *llmtest.Conn) *auth.User {
	return &auth.User{UserMeta: auth.UserMeta{Groups: append([]string(nil), conn.Config.Groups...)}}
}

func integrationModelName(t *testing.T, m *Manager, provider string, user *auth.User, preferred string) string {
	t.Helper()

	models, err := m.ListModels(context.Background(), schema.ModelListRequest{Provider: provider}, user)
	if isIntegrationUnreachable(err) {
		t.Skipf("provider unreachable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(models.Body) == 0 {
		t.Skip("no models available, skipping")
	}
	if preferred != "" {
		return preferred
	}
	return models.Body[0].Name
}

func isIntegrationUnreachable(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}
