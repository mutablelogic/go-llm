package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// firstModel returns the name of the first model available on the server,
// or skips the test. Different Ollama servers have different models
// pulled - sometimes none at all - so no test may assume a specific name,
// or even that one exists.
func firstModel(t *testing.T, c *ollama.Client) string {
	t.Helper()
	models, err := c.ListModels(context.Background())
	require.NoError(t, err)
	if len(models) == 0 {
		t.Skip("no models available on this server, skipping")
	}
	return models[0].Name()
}

///////////////////////////////////////////////////////////////////////////////
// ListModels

func Test_ListModels(t *testing.T) {
	c := requireClient(t)

	models, err := c.ListModels(context.Background())
	require.NoError(t, err)

	for _, m := range models {
		assert.NotEmpty(t, m.Name())
		assert.Same(t, c, m.Provider())
	}
}

func Test_ListModels_ImplementsModel(t *testing.T) {
	c := requireClient(t)

	models, err := c.ListModels(context.Background())
	require.NoError(t, err)
	if len(models) == 0 {
		t.Skip("no models available on this server, skipping")
	}
	assert.Implements(t, (*llm.Model)(nil), models[0])
}

///////////////////////////////////////////////////////////////////////////////
// GetModel

func Test_GetModel_KnownModel(t *testing.T) {
	c := requireClient(t)
	name := firstModel(t, c)

	model, err := c.GetModel(context.Background(), name)
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, name, model.Name())
	assert.Same(t, c, model.Provider())
}

func Test_GetModel_NotFound(t *testing.T) {
	c := requireClient(t)

	_, err := c.GetModel(context.Background(), "nonexistent-model-xyz:latest")
	assert.Error(t, err)
}

func Test_GetModel_Meta(t *testing.T) {
	c := requireClient(t)
	name := firstModel(t, c)

	model, err := c.GetModel(context.Background(), name)
	require.NoError(t, err)
	require.NotNil(t, model)

	// GetModel's response (/api/show) includes a "details" object, so
	// Meta() shouldn't be empty here
	meta := model.Meta()
	assert.NotEmpty(t, meta)
	assert.Contains(t, meta, "details")
}

func Test_GetModel_InputTokenLimit(t *testing.T) {
	c := requireClient(t)
	name := firstModel(t, c)

	model, err := c.GetModel(context.Background(), name)
	require.NoError(t, err)
	require.NotNil(t, model)

	// Ollama's /api/show response includes a "<family>.context_length" key
	// in model_info for every model family seen in practice, but this isn't
	// documented as guaranteed, so this only logs rather than asserts.
	limit := model.InputTokenLimit()
	if limit != nil {
		t.Logf("model %s: input token limit = %d", model.Name(), *limit)
	} else {
		t.Logf("model %s: input token limit = <none>", model.Name())
	}
}

///////////////////////////////////////////////////////////////////////////////
// STUB SERVER

// newStubClient returns a client for a stub Ollama server, with a vision
// model which has more than one context length, a model which can't be
// shown, and a model which isn't tagged "latest"
func newStubClient(t *testing.T) *ollama.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tags":
			w.Write([]byte(`{"models":[{"name":"llama3.2-vision:latest","details":{}},{"name":"broken:latest","details":{"family":"broken"}},{"name":"qwen3:8b","details":{}}]}`))
		case "/api/show":
			var req struct {
				Name string `json:"name"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if req.Name != "llama3.2-vision:latest" {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":"model not found"}`))
				return
			}
			w.Write([]byte(`{"details":{"family":"mllama"},"capabilities":["completion","vision"],"model_info":{"clip.context_length":2048,"mllama.context_length":131072}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	c, err := ollama.New(server.URL)
	require.NoError(t, err)
	return c
}

func Test_ListModels_Show(t *testing.T) {
	// Listing shows each model, so the models have their context length
	models, err := newStubClient(t).ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 3)

	// The context length of the model's family is used, not the projector's
	vision := models[0]
	assert.Equal(t, "llama3.2-vision:latest", vision.Name())
	require.NotNil(t, vision.InputTokenLimit())
	assert.Equal(t, uint(131072), *vision.InputTokenLimit())

	// A model which can't be shown is listed without it
	broken := models[1]
	assert.Equal(t, "broken:latest", broken.Name())
	assert.Nil(t, broken.InputTokenLimit())
}

func Test_ListModels_Capabilities(t *testing.T) {
	// A model has the capabilities Ollama reports, and a model which can't
	// be shown has none
	models, err := newStubClient(t).ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 3)
	assert.Equal(t, llm.ModelCap{Chat: types.Ptr(true), Tools: types.Ptr(false), Thinking: types.Ptr(false), ImageInput: types.Ptr(true), AudioInput: types.Ptr(false), VideoInput: types.Ptr(false), DocumentInput: types.Ptr(false), Embedding: types.Ptr(false), SpeechOutput: types.Ptr(false), Temperature: types.Ptr(true)}, models[0].Capabilities(), "a model which replies is only sent images")
	assert.Equal(t, llm.ModelCap{}, models[1].Capabilities())
}

func Test_ListModels_Aliases(t *testing.T) {
	// A model tagged "latest" has its name without the tag as an alias, as
	// Ollama uses the tag when there's none
	models, err := newStubClient(t).ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 3)
	assert.Equal(t, []string{"llama3.2-vision"}, models[0].Aliases())
	assert.Empty(t, models[2].Aliases())
}

func Test_GetModel_FamilyContextLength(t *testing.T) {
	// The context length of the model's family is preferred, every time
	c := newStubClient(t)
	for range 10 {
		model, err := c.GetModel(context.Background(), "llama3.2-vision:latest")
		require.NoError(t, err)
		require.NotNil(t, model.InputTokenLimit())
		assert.Equal(t, uint(131072), *model.InputTokenLimit())
	}
}
