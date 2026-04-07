package manager

import (
	"context"
	"testing"
	"time"

	// Packages
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

	ctx := llmtest.Context(t)
	m, err := New(ctx, "test", "0.0.0", conn, WithPassphrase(1, "test1234"))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Exec(ctx, `TRUNCATE llm.provider CASCADE`); err != nil {
		t.Fatal(err)
	}
	llmtest.RunBackground(t, func(ctx context.Context) error {
		return m.Run(ctx, llmtest.DiscardLogger())
	})
	llmtest.WaitUntil(t, 5*time.Second, func() bool {
		return m.Toolkit != nil
	}, "timed out waiting for llmmanager Run to initialize toolkit")

	return conn, m
}
