package test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	uuid "github.com/google/uuid"
	mock "github.com/mutablelogic/go-llm/pkg/mcp/mock"
	mcpserver "github.com/mutablelogic/go-llm/pkg/mcp/server"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

// Context returns a bounded context suitable for integration tests.
func Context(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

// DiscardLogger returns a logger that drops all output.
func DiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// RunBackground runs a cancelable integration-test loop and waits for it to
// exit during test cleanup.
func RunBackground(t *testing.T, run func(context.Context) error) {
	t.Helper()
	runCtx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() {
		runErr <- run(runCtx)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-runErr; err != nil && !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
	})
}

// WaitUntil polls until the condition becomes true or the timeout elapses.
func WaitUntil(t *testing.T, duration time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

// AdminUser returns a synthetic user with the configured provider groups.
func AdminUser(conn *Conn) *auth.User {
	return User(conn, conn.Config.Groups...)
}

// User creates a synthetic auth user and optional group memberships.
func User(conn *Conn, groups ...string) *auth.User {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id := uuid.New()
	if err := conn.Exec(ctx, fmt.Sprintf(`INSERT INTO auth."user" ("id") VALUES ('%s')`, id)); err != nil {
		fatalConnf(conn, "insert integration user: %v", err)
	}
	for _, group := range groups {
		if err := conn.Exec(ctx, fmt.Sprintf(`INSERT INTO auth.user_group ("user", "group") VALUES ('%s', '%s')`, id, group)); err != nil {
			fatalConnf(conn, "insert integration user group %q: %v", group, err)
		}
	}

	return &auth.User{
		ID: auth.UserID(id),
		UserMeta: auth.UserMeta{
			Groups: append([]string(nil), groups...),
		},
	}
}

// IsUnreachable reports transport-level connectivity failures for live-provider
// integration tests.
func IsUnreachable(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}

// CreateProvider persists and syncs a provider for an integration test.
func CreateProvider(
	t *testing.T,
	insert schema.ProviderInsert,
	create func(context.Context, schema.ProviderInsert) (*schema.Provider, error),
	sync func(context.Context) ([]string, []string, error),
) *schema.Provider {
	t.Helper()
	ctx := Context(t)

	provider, err := create(ctx, insert)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sync(ctx); err != nil {
		t.Fatal(err)
	}

	return provider
}

// ConnectorURL creates a live in-process MCP server and returns its base URL.
func ConnectorURL(t *testing.T, name string) string {
	t.Helper()

	srv, err := mcpserver.New(name, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.AddTools(&mock.MockTool{Name_: "remote_tool", Description_: "A remote tool"}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts.URL
}

// ModelName returns the preferred model when available, otherwise the first
// model advertised by the provider.
func ModelName(
	t *testing.T,
	preferred string,
	list func(context.Context) (*schema.ModelList, error),
) string {
	t.Helper()
	ctx := Context(t)

	models, err := list(ctx)
	if IsUnreachable(err) {
		t.Skipf("provider unreachable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(models.Body) == 0 {
		t.Skip("no models available, skipping")
	}
	if preferred != "" {
		for _, model := range models.Body {
			if model.Name == preferred {
				return preferred
			}
		}
	}
	return models.Body[0].Name
}

// ModelNameMatching returns the first model that matches the predicate and,
// when provided, passes validation. The preferred model is tried first.
func ModelNameMatching(
	t *testing.T,
	preferred string,
	list func(context.Context) (*schema.ModelList, error),
	match func(schema.Model) bool,
	validate func(context.Context, string) error,
) string {
	t.Helper()
	ctx := Context(t)

	var models *schema.ModelList
	var err error
	deadline := time.Now().Add(5 * time.Second)
	for {
		models, err = list(ctx)
		if err == nil {
			break
		}
		if IsUnreachable(err) {
			t.Skipf("provider unreachable: %v", err)
		}
		if !errors.Is(err, schema.ErrNotFound) && !strings.Contains(err.Error(), "provider ") {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(models.Body) == 0 {
		t.Skip("no models available, skipping")
	}

	candidates := make([]string, 0, len(models.Body)+1)
	if preferred != "" {
		candidates = append(candidates, preferred)
	}
	for _, model := range models.Body {
		if match == nil || match(model) {
			candidates = append(candidates, model.Name)
		}
	}
	if len(candidates) == 0 {
		for _, model := range models.Body {
			candidates = append(candidates, model.Name)
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	firstErr := error(nil)
	for _, candidate := range candidates {
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		if validate == nil {
			return candidate
		}
		if err := validate(ctx, candidate); err != nil {
			if IsUnreachable(err) {
				t.Skipf("provider unreachable: %v", err)
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		return candidate
	}

	if firstErr != nil {
		t.Fatal(firstErr)
	}
	return models.Body[0].Name
}

func fatalConnf(conn *Conn, format string, args ...any) {
	if conn != nil && conn.t != nil {
		conn.t.Helper()
		conn.t.Fatalf(format, args...)
	}
	panic(fmt.Sprintf(format, args...))
}
