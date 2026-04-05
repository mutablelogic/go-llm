package test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	pgtest "github.com/mutablelogic/go-pg/pkg/test"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ProviderConfig struct {
	Name     string
	Provider string
	Model    string
	URL      string
	APIKey   string
	Groups   []string
}

type Conn struct {
	pg.PoolConn
	Config     ProviderConfig
	SkipReason string
	t          *testing.T
}

const timeout = 2 * time.Minute

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func Main(m *testing.M, conn *Conn, config ProviderConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	name, err := os.Executable()
	if err != nil {
		panic(err)
	}

	verbose := slices.Contains(os.Args, "-test.v=true")
	container, pool, err := pgtest.NewPgxContainer(ctx, filepath.Base(name), verbose, func(ctx context.Context, sql string, args any, err error) {
		if err != nil {
			log.Printf("ERROR: %v", err)
		}
		if verbose || err != nil {
			if args == nil {
				log.Printf("SQL: %v", sql)
			} else {
				log.Printf("SQL: %v, ARGS: %v", sql, args)
			}
		}
	})
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	defer container.Close(ctx)

	resolved, skipReason, err := resolveProviderConfig(config)
	if err != nil {
		panic(err)
	}
	if err := bootstrapAuth(ctx, pool, resolved.Groups); err != nil {
		panic(err)
	}

	*conn = Conn{PoolConn: pool, Config: resolved, SkipReason: skipReason}
	os.Exit(m.Run())
}

func (c *Conn) Begin(t *testing.T) *Conn {
	t.Log("Begin", t.Name())
	return &Conn{PoolConn: c.PoolConn, Config: c.Config, SkipReason: c.SkipReason, t: t}
}

func (c *Conn) Close() {
	if c.t != nil {
		c.t.Log("End", c.t.Name())
	}
}

func (c *Conn) RequireProvider(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if c.SkipReason != "" {
		t.Skip(c.SkipReason)
	}
}

func (c *Conn) ProviderInsert() schema.ProviderInsert {
	enabled := true
	return schema.ProviderInsert{
		Name:     c.Config.Name,
		Provider: c.Config.Provider,
		ProviderMeta: schema.ProviderMeta{
			URL:     stringPtr(c.Config.URL),
			Enabled: &enabled,
			Groups:  append([]string(nil), c.Config.Groups...),
		},
		ProviderCredentials: schema.ProviderCredentials{
			APIKey: c.Config.APIKey,
		},
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func resolveProviderConfig(config ProviderConfig) (ProviderConfig, string, error) {
	resolved := config
	if resolved.Provider == "" {
		return ProviderConfig{}, "", fmt.Errorf("provider is required")
	}
	if resolved.Name == "" {
		resolved.Name = resolved.Provider + "-integration"
	}

	switch resolved.Provider {
	case schema.Anthropic:
		if resolved.APIKey == "" {
			resolved.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if resolved.APIKey == "" {
			return resolved, "ANTHROPIC_API_KEY not set, skipping", nil
		}
	case schema.Gemini:
		if resolved.APIKey == "" {
			resolved.APIKey = os.Getenv("GEMINI_API_KEY")
		}
		if resolved.APIKey == "" {
			resolved.APIKey = os.Getenv("GOOGLE_API_KEY")
		}
		if resolved.APIKey == "" {
			return resolved, "GEMINI_API_KEY not set, skipping", nil
		}
	case schema.Mistral:
		if resolved.APIKey == "" {
			resolved.APIKey = os.Getenv("MISTRAL_API_KEY")
		}
		if resolved.APIKey == "" {
			return resolved, "MISTRAL_API_KEY not set, skipping", nil
		}
	case schema.Ollama:
		if resolved.URL == "" {
			resolved.URL = os.Getenv("OLLAMA_URL")
		}
		if resolved.URL == "" {
			return resolved, "OLLAMA_URL not set, skipping", nil
		}
	case schema.Eliza:
		// No provider-specific env required.
	default:
		return ProviderConfig{}, "", fmt.Errorf("unsupported provider %q", resolved.Provider)
	}

	return resolved, "", nil
}

func bootstrapAuth(ctx context.Context, conn pg.Conn, groups []string) error {
	statements := []string{
		`CREATE SCHEMA IF NOT EXISTS auth`,
		`CREATE TABLE IF NOT EXISTS auth."group" ("id" TEXT PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS auth."user" ("id" UUID PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS auth.user_group ("user" UUID NOT NULL REFERENCES auth."user" ("id") ON DELETE CASCADE, "group" TEXT NOT NULL REFERENCES auth."group" ("id") ON DELETE CASCADE, PRIMARY KEY ("user", "group"))`,
	}
	for _, statement := range statements {
		if err := conn.Exec(ctx, statement); err != nil {
			return err
		}
	}
	for _, group := range groups {
		if err := conn.Exec(ctx, fmt.Sprintf(`INSERT INTO auth."group" ("id") VALUES ('%s') ON CONFLICT DO NOTHING`, group)); err != nil {
			return err
		}
	}
	return nil
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
