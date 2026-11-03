package httpclient_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// stubOllama is a stub Ollama server, whose models are pulled, deleted and
// unloaded
type stubOllama struct {
	sync.Mutex
	models   []string
	unloaded []string
}

func (s *stubOllama) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model  string `json:"model"`
		Name   string `json:"name"`
		Stream bool   `json:"stream"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Model == "" {
		req.Model = req.Name
	}
	s.Lock()
	defer s.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/version":
		w.Write([]byte(`{"version":"0.0.1"}`))
	case "/api/tags":
		var models []map[string]any
		for _, name := range s.models {
			models = append(models, map[string]any{"name": name, "details": map[string]any{}})
		}
		json.NewEncoder(w).Encode(map[string]any{"models": models})
	case "/api/show":
		if !slices.Contains(s.models, req.Model) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"model not found"}`))
			return
		}
		w.Write([]byte(`{"details":{"family":"llama"},"model_info":{}}`))
	case "/api/pull":
		if req.Model == "missing:latest" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"pull model manifest: file does not exist"}`))
			return
		}
		s.models = append(s.models, req.Model)
		if req.Stream {
			w.Header().Set("Content-Type", "application/x-ndjson")
			for _, completed := range []int{50, 100} {
				fmt.Fprintf(w, `{"status":"pulling","total":100,"completed":%d}`+"\n", completed)
			}
			w.Write([]byte(`{"status":"success"}` + "\n"))
			return
		}
		w.Write([]byte(`{"status":"success"}`))
	case "/api/generate":
		s.unloaded = append(s.unloaded, req.Model)
		w.Write([]byte(`{"done":true,"done_reason":"unload"}`))
	case "/api/delete":
		s.models = slices.DeleteFunc(s.models, func(name string) bool { return name == req.Model })
	default:
		http.NotFound(w, r)
	}
}

// newLoadClient returns a client of a test server of a registry, with
// eliza, and a stub Ollama server, as home, which has a model
func newLoadClient(t *testing.T) (*httpclient.Client, *stubOllama) {
	t.Helper()
	stub := &stubOllama{models: []string{"llama3.2:latest", "x/flux2-klein:latest"}}
	ollama := httptest.NewServer(stub)
	t.Cleanup(ollama.Close)
	client := newClient(t)
	_, err := client.CreateProvider(t.Context(), schema.Provider{Name: "home", Type: llm.Ollama, URL: ollama.URL})
	require.NoError(t, err)
	return client, stub
}

func Test_LoadModel(t *testing.T) {
	ctx := t.Context()
	client, stub := newLoadClient(t)

	t.Run("Load", func(t *testing.T) {
		// A model is loaded with the one provider which loads models, and
		// is then listed
		model, err := client.LoadModel(ctx, schema.LoadModelRequest{Name: "phi4"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "home:phi4:latest", model.Canonical)
		assert.Contains(t, stub.models, "phi4:latest")
		_, err = client.GetModel(ctx, "phi4:latest")
		require.NoError(t, err)

		// And it's got without its tag, as it's "latest", as it's loaded
		for _, name := range []string{"phi4", "home:phi4"} {
			model, err := client.GetModel(ctx, name)
			require.NoError(t, err)
			assert.Equal(t, "home:phi4:latest", model.Canonical)
		}
	})

	t.Run("Progress", func(t *testing.T) {
		// The download's progress is streamed
		var percents []float64
		model, err := client.LoadModel(ctx, schema.LoadModelRequest{Provider: "home", Name: "gemma"}, func(_ string, percent float64) {
			percents = append(percents, percent)
		})
		require.NoError(t, err)
		assert.Equal(t, "home:gemma:latest", model.Canonical)
		assert.Equal(t, []float64{50, 100, 0}, percents)
	})

	t.Run("Errors", func(t *testing.T) {
		// A model which can't be loaded, as a response, or in a stream
		_, err := client.LoadModel(ctx, schema.LoadModelRequest{Name: "missing"}, nil)
		assert.Equal(t, http.StatusServiceUnavailable, status(t, err))
		_, err = client.LoadModel(ctx, schema.LoadModelRequest{Name: "missing"}, func(string, float64) {})
		assert.ErrorContains(t, err, "file does not exist")

		// A provider which doesn't exist, or can't load models
		_, err = client.LoadModel(ctx, schema.LoadModelRequest{Provider: "away", Name: "phi4"}, nil)
		assert.Equal(t, http.StatusNotFound, status(t, err))
		_, err = client.LoadModel(ctx, schema.LoadModelRequest{Provider: "eliza", Name: "phi4"}, nil)
		assert.Equal(t, http.StatusNotImplemented, status(t, err))

		// And a name is needed
		_, err = client.LoadModel(ctx, schema.LoadModelRequest{}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_DeleteModel(t *testing.T) {
	ctx := t.Context()
	client, stub := newLoadClient(t)

	t.Run("Delete", func(t *testing.T) {
		// A model is deleted with its provider, and isn't listed after
		model, err := client.DeleteModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		assert.Equal(t, "home:llama3.2:latest", model.Canonical)
		assert.NotContains(t, stub.models, "llama3.2:latest")
		_, err = client.GetModel(ctx, "llama3.2:latest")
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})

	t.Run("Slash", func(t *testing.T) {
		// A model whose name has a "/", which is escaped in the path, is got,
		// and deleted
		model, err := client.GetModel(ctx, "x/flux2-klein:latest")
		require.NoError(t, err)
		assert.Equal(t, "home:x/flux2-klein:latest", model.Canonical)
		model, err = client.DeleteModel(ctx, "home:x/flux2-klein:latest")
		require.NoError(t, err)
		assert.Equal(t, "x/flux2-klein:latest", model.Name)
		assert.NotContains(t, stub.models, "x/flux2-klein:latest")
	})

	t.Run("Errors", func(t *testing.T) {
		_, err := client.DeleteModel(ctx, "missing")
		assert.Equal(t, http.StatusNotFound, status(t, err))
		_, err = client.DeleteModel(ctx, "eliza")
		assert.Equal(t, http.StatusNotImplemented, status(t, err))
		_, err = client.DeleteModel(ctx, " ")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_UnloadModel(t *testing.T) {
	ctx := t.Context()
	client, stub := newLoadClient(t)

	t.Run("Unload", func(t *testing.T) {
		// A model is unloaded with its provider, and is still stored
		model, err := client.UnloadModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		assert.Equal(t, "home:llama3.2:latest", model.Canonical)
		assert.Equal(t, []string{"llama3.2:latest"}, stub.unloaded)
		_, err = client.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
	})

	t.Run("Latest", func(t *testing.T) {
		// A model which is tagged "latest" is unloaded without its tag
		model, err := client.UnloadModel(ctx, "home:llama3.2")
		require.NoError(t, err)
		assert.Equal(t, "home:llama3.2:latest", model.Canonical)
		assert.Equal(t, "llama3.2:latest", stub.unloaded[len(stub.unloaded)-1])
	})

	t.Run("Slash", func(t *testing.T) {
		// A model whose name has a "/", which is escaped in the path, and
		// whose tag is "latest"
		model, err := client.UnloadModel(ctx, "home:x/flux2-klein")
		require.NoError(t, err)
		assert.Equal(t, "home:x/flux2-klein:latest", model.Canonical)
		assert.Contains(t, stub.unloaded, "x/flux2-klein:latest")
	})

	t.Run("Errors", func(t *testing.T) {
		_, err := client.UnloadModel(ctx, "missing")
		assert.Equal(t, http.StatusNotFound, status(t, err))
		_, err = client.UnloadModel(ctx, "eliza")
		assert.Equal(t, http.StatusNotImplemented, status(t, err))
		_, err = client.UnloadModel(ctx, " ")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}
