package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// downloadServer is a stub Ollama server, which can pull, delete and
// unload models, and records the models deleted, and unloaded
type downloadServer struct {
	sync.Mutex
	deleted  []string
	unloaded []string
}

func (s *downloadServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model     string `json:"model"`
		Name      string `json:"name"`
		Stream    bool   `json:"stream"`
		KeepAlive *int   `json:"keep_alive"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/pull":
		switch {
		case req.Model == "missing:latest" && req.Stream:
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Write([]byte("{\"status\":\"pulling manifest\"}\n{\"error\":\"pull model manifest: file does not exist\"}\n"))
		case req.Model == "missing:latest":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"pull model manifest: file does not exist"}`))
		case req.Stream:
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Write([]byte("{\"status\":\"pulling manifest\"}\n" +
				"{\"status\":\"pulling abc\",\"digest\":\"abc\",\"total\":200,\"completed\":50}\n" +
				"{\"status\":\"pulling abc\",\"digest\":\"abc\",\"total\":200,\"completed\":200}\n" +
				"{\"status\":\"success\"}\n"))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"success"}`))
		}
	case r.Method == http.MethodPost && r.URL.Path == "/api/show":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"details":{"family":"llama"},"model_info":{"llama.context_length":131072}}`))
	case r.Method == http.MethodPost && r.URL.Path == "/api/generate" && req.KeepAlive != nil && *req.KeepAlive == 0 && !req.Stream:
		s.Lock()
		defer s.Unlock()
		s.unloaded = append(s.unloaded, req.Model)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"` + req.Model + `","done":true,"done_reason":"unload"}`))
	case r.Method == http.MethodDelete && r.URL.Path == "/api/delete":
		s.Lock()
		defer s.Unlock()
		s.deleted = append(s.deleted, req.Model)
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func newDownloadClient(t *testing.T) (*ollama.Client, *downloadServer) {
	t.Helper()
	stub := new(downloadServer)
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := ollama.New(server.URL)
	require.NoError(t, err)
	return c, stub
}

func Test_ImplementsDownloader(t *testing.T) {
	assert.Implements(t, (*llm.Downloader)(nil), &ollama.Client{})
}

func Test_LoadModel(t *testing.T) {
	c, _ := newDownloadClient(t)
	ctx := context.Background()

	t.Run("Progress", func(t *testing.T) {
		// The progress function is called with each status, and the
		// percentage downloaded
		type update struct {
			status  string
			percent float64
		}
		var updates []update
		model, err := c.LoadModel(ctx, "llama3.2:latest", func(status string, percent float64) {
			updates = append(updates, update{status, percent})
		})
		require.NoError(t, err)
		assert.Equal(t, []update{
			{"pulling manifest", 0},
			{"pulling abc", 25},
			{"pulling abc", 100},
			{"success", 0},
		}, updates)

		// The downloaded model is returned
		assert.Equal(t, "llama3.2:latest", model.Name())
		require.NotNil(t, model.InputTokenLimit())
		assert.Equal(t, uint(131072), *model.InputTokenLimit())
	})

	t.Run("NoProgress", func(t *testing.T) {
		model, err := c.LoadModel(ctx, "llama3.2:latest", nil)
		require.NoError(t, err)
		assert.Equal(t, "llama3.2:latest", model.Name())
	})

	t.Run("Error", func(t *testing.T) {
		// An error is returned, with or without progress
		_, err := c.LoadModel(ctx, "missing", func(string, float64) {})
		assert.ErrorContains(t, err, "file does not exist")
		_, err = c.LoadModel(ctx, "missing", nil)
		assert.Error(t, err)
	})

	t.Run("Cancelled", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, err := c.LoadModel(cancelled, "llama3.2:latest", nil)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func Test_DeleteModel(t *testing.T) {
	c, stub := newDownloadClient(t)
	ctx := context.Background()

	t.Run("Delete", func(t *testing.T) {
		model, err := c.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		require.NoError(t, c.DeleteModel(ctx, model))
		assert.Equal(t, []string{"llama3.2:latest"}, stub.deleted)
	})

	t.Run("NotOllama", func(t *testing.T) {
		// A model from another provider isn't deleted
		other, err := eliza.New()
		require.NoError(t, err)
		model, err := other.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		assert.ErrorIs(t, c.DeleteModel(ctx, model), llm.ErrBadParameter)
		assert.ErrorIs(t, c.DeleteModel(ctx, nil), llm.ErrBadParameter)
	})
}

func Test_UnloadModel(t *testing.T) {
	c, stub := newDownloadClient(t)
	ctx := context.Background()

	t.Run("Unload", func(t *testing.T) {
		// The model is unloaded, with a request to generate with it, which
		// keeps it loaded for no time, and isn't deleted
		model, err := c.GetModel(ctx, "llama3.2:latest")
		require.NoError(t, err)
		require.NoError(t, c.UnloadModel(ctx, model))
		assert.Equal(t, []string{"llama3.2:latest"}, stub.unloaded)
		assert.Empty(t, stub.deleted)
	})

	t.Run("NotOllama", func(t *testing.T) {
		// A model from another provider isn't unloaded
		other, err := eliza.New()
		require.NoError(t, err)
		model, err := other.GetModel(ctx, eliza.ModelName)
		require.NoError(t, err)
		assert.ErrorIs(t, c.UnloadModel(ctx, model), llm.ErrBadParameter)
		assert.ErrorIs(t, c.UnloadModel(ctx, nil), llm.ErrBadParameter)
	})
}
