package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_UpdateProvider(t *testing.T) {
	ctx := context.Background()
	t.Setenv("OLLAMA_KEY", "secret")
	r, err := registry.New()
	require.NoError(t, err)
	ch := watch(t, r)

	t.Run("Update", func(t *testing.T) {
		// A provider's fields are replaced, and it's created again, and
		// updated, for watchers
		created, err := r.CreateProvider(ctx, schema.Provider{Name: "chat", Type: llm.Eliza, Serial: 10})
		require.NoError(t, err)
		assert.Equal(t, uint64(1), created.Serial)
		first := r.Provider("chat")
		provider, err := r.UpdateProvider(ctx, "chat", schema.ProviderMeta{Exclude: []string{"-de$"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"-de$"}, provider.Exclude)
		assert.Equal(t, uint64(2), provider.Serial)
		assert.NotSame(t, first, r.Provider("chat"))
		_, err = r.GetModel(ctx, "chat:eliza-1966-de")
		assert.ErrorIs(t, err, llm.ErrNotFound)

		// And replaced again, so patterns which aren't given are cleared
		provider, err = r.UpdateProvider(ctx, "chat", schema.ProviderMeta{})
		require.NoError(t, err)
		assert.Empty(t, provider.Exclude)
		_, err = r.GetModel(ctx, "chat:eliza-1966-de")
		require.NoError(t, err)
		assert.Equal(t, []eventschema.Event{
			{Action: eventschema.EventCreate, Schema: registry.EventSchema, Table: registry.ProviderTable, Row: "chat"},
			{Action: eventschema.EventUpdate, Schema: registry.EventSchema, Table: registry.ProviderTable, Row: "chat"},
			{Action: eventschema.EventUpdate, Schema: registry.EventSchema, Table: registry.ProviderTable, Row: "chat"},
		}, events(ch))
	})

	t.Run("Same", func(t *testing.T) {
		// A provider whose fields are the same is left as it is
		current := r.Provider("chat")
		provider, err := r.UpdateProvider(ctx, "chat", schema.ProviderMeta{})
		require.NoError(t, err)
		assert.Same(t, current, r.Provider("chat"))
		assert.Equal(t, uint64(3), provider.Serial)
		assert.Empty(t, events(ch))
	})

	t.Run("Key", func(t *testing.T) {
		// An API key which isn't given is kept, as it isn't returned, and one
		// which is given replaces it
		var keys []string
		ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keys = append(keys, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"0.0.1"}`))
		}))
		t.Cleanup(ollama.Close)
		_, err := r.CreateProvider(ctx, schema.Provider{Name: "home", Type: llm.Ollama, URL: ollama.URL, APIKey: "${OLLAMA_KEY}"})
		require.NoError(t, err)
		_, err = r.UpdateProvider(ctx, "home", schema.ProviderMeta{URL: ollama.URL, Include: []string{"^llama"}})
		require.NoError(t, err)
		_, err = r.UpdateProvider(ctx, "home", schema.ProviderMeta{URL: ollama.URL, Include: []string{"^llama"}, APIKey: "other"})
		require.NoError(t, err)
		assert.Equal(t, []string{"Bearer secret", "Bearer secret", "Bearer other"}, keys)
		events(ch)
	})

	t.Run("Password", func(t *testing.T) {
		// A URL with a password, which the API returns redacted, is kept,
		// when an update sends it back, as it is, such as to change only the
		// patterns, and replaced, when another is sent
		var passwords []string
		ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, password, _ := r.BasicAuth()
			passwords = append(passwords, password)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"0.0.1"}`))
		}))
		t.Cleanup(ollama.Close)
		withPassword := strings.Replace(ollama.URL, "http://", "http://david:secret@", 1)
		created, err := r.CreateProvider(ctx, schema.Provider{Name: "proxied", Type: llm.Ollama, URL: withPassword})
		require.NoError(t, err)
		assert.NotContains(t, created.URL, "secret", "it's returned redacted")
		_, err = r.UpdateProvider(ctx, "proxied", schema.ProviderMeta{URL: created.URL, Include: []string{"^llama"}})
		require.NoError(t, err)
		_, err = r.UpdateProvider(ctx, "proxied", schema.ProviderMeta{URL: strings.Replace(withPassword, "secret", "other", 1), Include: []string{"^llama"}})
		require.NoError(t, err)
		require.NotEmpty(t, passwords)
		assert.NotContains(t, passwords, "xxxxx", "the redacted password is never sent")
		assert.Equal(t, "other", passwords[len(passwords)-1])
		assert.Contains(t, passwords, "secret")
	})

	t.Run("Unreachable", func(t *testing.T) {
		// A provider which can't be reached once it's updated is updated,
		// with its error
		closed := httptest.NewServer(http.NotFoundHandler())
		closed.Close()
		provider, err := r.UpdateProvider(ctx, "home", schema.ProviderMeta{URL: closed.URL})
		require.NoError(t, err)
		assert.NotEmpty(t, provider.Error)
		events(ch)
	})

	t.Run("Errors", func(t *testing.T) {
		// A provider which doesn't exist, or was added without a
		// definition, isn't updated, and the fields must be valid
		_, err := r.UpdateProvider(ctx, "missing", schema.ProviderMeta{})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		client, err := eliza.New()
		require.NoError(t, err)
		require.NoError(t, r.Add("added", client))
		added, err := r.GetProvider(ctx, "added")
		require.NoError(t, err)
		assert.Zero(t, added.Serial)
		_, err = r.UpdateProvider(ctx, "added", schema.ProviderMeta{})
		assert.ErrorIs(t, err, llm.ErrConflict)
		_, err = r.UpdateProvider(ctx, "chat", schema.ProviderMeta{Include: []string{"("}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.UpdateProvider(ctx, "chat", schema.ProviderMeta{URL: "http://localhost"})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}
