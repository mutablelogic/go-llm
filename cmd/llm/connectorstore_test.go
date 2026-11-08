//go:build !client

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	mcpregistry "github.com/mutablelogic/go-llm/mcp/registry"
	mcpschema "github.com/mutablelogic/go-llm/mcp/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	oauth2 "golang.org/x/oauth2"
)

func Test_ConnectorStore(t *testing.T) {
	ctx := context.Background()
	connector := func(name, url string, serial uint64) mcpschema.Connector {
		return mcpschema.Connector{Name: name, ConnectorMeta: mcpschema.ConnectorMeta{URL: url}, Serial: serial}
	}
	newStore := func(t *testing.T, path string) mcpregistry.Store {
		t.Helper()
		file, err := newJSONFile(path)
		require.NoError(t, err)
		store, err := newConnectorStore(file)
		require.NoError(t, err)
		return store
	}

	t.Run("Kept", func(t *testing.T) {
		// Connectors are kept in the file, in order of their names, without
		// their status, so another store of the file, such as once the
		// server restarts, has them
		path := filepath.Join(t.TempDir(), connectorsFile)
		store := newStore(t, path)
		status := connector("search", "https://mcp.example.com/search", 0)
		status.Connected, status.Error = true, "fake"
		created, err := store.Put(ctx, status)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), created.Serial)
		assert.False(t, created.Connected)
		_, err = store.Put(ctx, connector("github", "https://mcp.example.com/github", 0))
		require.NoError(t, err)

		list, err := newStore(t, path).List(ctx)
		require.NoError(t, err)
		require.Len(t, list, 2)
		assert.Equal(t, "github", list[0].Name)
		assert.Equal(t, "search", list[1].Name)
		assert.Equal(t, list[1].Definition(), list[1])
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("Credentials", func(t *testing.T) {
		// How a connector's server is authorized, such as its token, is kept
		// in the file, beside the connectors, so another store of it has it
		path := filepath.Join(t.TempDir(), connectorsFile)
		file, err := newJSONFile(path)
		require.NoError(t, err)
		store, err := newConnectorStore(file)
		require.NoError(t, err)
		_, err = store.GetCredential(ctx, "github")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = store.Put(ctx, connector("github", "https://mcp.example.com/github", 0))
		require.NoError(t, err)
		credential := mcpschema.Credential{Issuer: "https://github.com/login/oauth", ClientID: "client", RedirectURIs: []string{"http://localhost/cb"}, Token: &oauth2.Token{AccessToken: "access", RefreshToken: "refresh"}}
		require.NoError(t, store.PutCredential(ctx, "github", credential))

		file, err = newJSONFile(path)
		require.NoError(t, err)
		other, err := newConnectorStore(file)
		require.NoError(t, err)
		got, err := other.GetCredential(ctx, "github")
		require.NoError(t, err)
		assert.Equal(t, "client", got.ClientID)
		assert.Equal(t, []string{"http://localhost/cb"}, got.RedirectURIs)
		require.NotNil(t, got.Token)
		assert.Equal(t, "refresh", got.Token.RefreshToken)
		list, err := other.List(ctx)
		require.NoError(t, err)
		assert.Len(t, list, 1, "the connectors are kept beside it")

		// And deleted
		require.NoError(t, other.DeleteCredential(ctx, "github"))
		require.NoError(t, other.DeleteCredential(ctx, "github"))
		_, err = store.GetCredential(ctx, "github")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Serials", func(t *testing.T) {
		// A connector is replaced, and deleted, with its serial, and not
		// another
		store := newStore(t, filepath.Join(t.TempDir(), connectorsFile))
		created, err := store.Put(ctx, connector("search", "https://mcp.example.com/search", 0))
		require.NoError(t, err)
		_, err = store.Put(ctx, connector("search", "https://mcp.example.com/other", 0))
		assert.ErrorIs(t, err, llm.ErrConflict)
		created.URL = "https://mcp.example.com/v2"
		updated, err := store.Put(ctx, created)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), updated.Serial)
		assert.ErrorIs(t, store.Delete(ctx, "search", 1), llm.ErrConflict)
		require.NoError(t, store.Delete(ctx, "search", 2))
		require.NoError(t, store.Delete(ctx, "search", 2))
		list, err := store.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
		_, err = store.Put(ctx, mcpschema.Connector{})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Recreated", func(t *testing.T) {
		// A serial isn't used again for a name, once its connector is
		// deleted, even once the server restarts
		path := filepath.Join(t.TempDir(), connectorsFile)
		store := newStore(t, path)
		first, err := store.Put(ctx, connector("search", "https://mcp.example.com/one", 0))
		require.NoError(t, err)
		require.NoError(t, store.Delete(ctx, "search", first.Serial))
		second, err := newStore(t, path).Put(ctx, connector("search", "https://mcp.example.com/two", 0))
		require.NoError(t, err)
		assert.Equal(t, uint64(2), second.Serial)
		_, err = newStore(t, path).Put(ctx, first)
		assert.ErrorIs(t, err, llm.ErrConflict)
	})

	t.Run("Corrupt", func(t *testing.T) {
		// A file which isn't JSON is an error, rather than having none
		path := filepath.Join(t.TempDir(), connectorsFile)
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))
		_, err := newStore(t, path).List(ctx)
		assert.Error(t, err)
	})

	t.Run("Registry", func(t *testing.T) {
		// A registry with the store keeps its connectors in the file
		path := filepath.Join(t.TempDir(), connectorsFile)
		r, err := mcpregistry.New(mcpregistry.WithStore(newStore(t, path)))
		require.NoError(t, err)
		_, err = r.CreateConnector(ctx, connector("search", "https://mcp.example.com/search", 0))
		require.NoError(t, err)
		again, err := mcpregistry.New(mcpregistry.WithStore(newStore(t, path)))
		require.NoError(t, err)
		got, err := again.GetConnector(ctx, "search")
		require.NoError(t, err)
		assert.Equal(t, "https://mcp.example.com/search", got.URL)
	})
}
