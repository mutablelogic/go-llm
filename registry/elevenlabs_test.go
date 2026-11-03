package registry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	elevenlabs "github.com/mutablelogic/go-llm/provider/elevenlabs"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ElevenLabs(t *testing.T) {
	// A stub of ElevenLabs' API, which is pinged with its models
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	r, err := registry.New()
	require.NoError(t, err)
	ctx := context.Background()

	// A provider which transcribes, but doesn't download, and whose models
	// are the Scribe models
	client, err := elevenlabs.New(server.URL, "secret")
	require.NoError(t, err)
	require.NoError(t, r.Add("speech", client))
	status, err := r.GetProvider(ctx, "speech")
	require.NoError(t, err)
	assert.True(t, status.Transcriber)
	assert.False(t, status.Downloader)
	model, err := r.GetModel(ctx, "scribe_v2")
	require.NoError(t, err)
	assert.Equal(t, "speech:scribe_v2", r.Name(model))

	// And which needs an API key
	_, err = r.CreateProvider(ctx, schema.Provider{Name: "keyless", Type: llm.ElevenLabs})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
