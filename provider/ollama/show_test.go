package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	// Packages
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// stubTags is a stub Ollama server, whose models have digests, which can be
// changed, and which counts the models it's asked to show
type stubTags struct {
	sync.Mutex
	digests map[string]string
	shown   map[string]int
}

func (s *stubTags) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Lock()
	defer s.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/tags":
		var models []map[string]any
		for name, digest := range s.digests {
			models = append(models, map[string]any{"name": name, "digest": digest, "details": map[string]any{}})
		}
		json.NewEncoder(w).Encode(map[string]any{"models": models})
	case "/api/show":
		var req struct {
			Name string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		s.shown[req.Name]++
		w.Write([]byte(`{"details":{"family":"llama"},"capabilities":["completion"],"model_info":{"llama.context_length":4096}}`))
	default:
		http.NotFound(w, r)
	}
}

// Shown returns how many times each model was shown, and forgets them
func (s *stubTags) Shown() map[string]int {
	s.Lock()
	defer s.Unlock()
	shown := s.shown
	s.shown = make(map[string]int)
	return shown
}

// Set sets the models, by their names, with their digests
func (s *stubTags) Set(digests map[string]string) {
	s.Lock()
	defer s.Unlock()
	s.digests = digests
}

func Test_ListModels_Shown(t *testing.T) {
	ctx := context.Background()
	stub := &stubTags{shown: make(map[string]int)}
	stub.Set(map[string]string{"llama3.2:latest": "a1", "phi4:latest": "b1", "local:latest": ""})
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	c, err := ollama.New(server.URL)
	require.NoError(t, err)

	// list lists the models, which have what they were shown with
	list := func(t *testing.T) {
		t.Helper()
		models, err := c.ListModels(ctx)
		require.NoError(t, err)
		for _, model := range models {
			if assert.NotNil(t, model.InputTokenLimit(), model.Name()) {
				assert.Equal(t, uint(4096), *model.InputTokenLimit(), model.Name())
			}
		}
	}

	// Each model is shown when they're first listed
	list(t)
	assert.Equal(t, map[string]int{"llama3.2:latest": 1, "phi4:latest": 1, "local:latest": 1}, stub.Shown())

	// And not again, when they haven't changed, by their digests, apart from
	// a model without a digest
	list(t)
	assert.Equal(t, map[string]int{"local:latest": 1}, stub.Shown())

	// A model which changed is shown again
	stub.Set(map[string]string{"llama3.2:latest": "a2", "phi4:latest": "b1", "local:latest": ""})
	list(t)
	assert.Equal(t, map[string]int{"llama3.2:latest": 1, "local:latest": 1}, stub.Shown())

	// And a model which is deleted is forgotten, so it's shown when it's
	// pulled again
	stub.Set(map[string]string{"llama3.2:latest": "a2"})
	list(t)
	assert.Empty(t, stub.Shown())
	stub.Set(map[string]string{"llama3.2:latest": "a2", "phi4:latest": "b1"})
	list(t)
	assert.Equal(t, map[string]int{"phi4:latest": 1}, stub.Shown())
}
