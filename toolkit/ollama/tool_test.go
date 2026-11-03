package ollama_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/toolkit/ollama"
	httpclient "github.com/mutablelogic/go-llm/toolkit/ollama/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// request is a request received by the fake Ollama server
type request struct {
	path string
	body map[string]any
}

// newTools returns the tools, with a client for a fake Ollama server, and the
// requests the server received
func newTools(t *testing.T) (map[string]llm.Tool, *[]request) {
	t.Helper()
	var requests []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		} else if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(data, &body)
		requests = append(requests, request{path: r.URL.Path, body: body})

		// Ollama returns JSON with the content type "text/html"
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/web_search":
			w.Write([]byte(`{"results":[{"title":"Ollama","url":"https://ollama.com","content":"Get up and running with large language models."}]}`))
		case "/web_fetch":
			w.Write([]byte(`{"title":"Ollama","content":"Get up and running with large language models.","links":["https://ollama.com/download"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	tools, err := ollama.NewTools("test-key", client.OptEndpoint(server.URL))
	require.NoError(t, err)
	result := make(map[string]llm.Tool, len(tools))
	for _, tool := range tools {
		result[tool.Name()] = tool
	}
	return result, &requests
}

func TestNewTools(t *testing.T) {
	t.Run("MissingKey", func(t *testing.T) {
		tools, err := ollama.NewTools("")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.Nil(t, tools)
	})

	t.Run("Tools", func(t *testing.T) {
		tools, _ := newTools(t)
		assert.Len(t, tools, 2)
		for name, property := range map[string]string{"search": "query", "fetch": "url"} {
			tool, ok := tools[name]
			require.True(t, ok, name)
			assert.NotEmpty(t, tool.Description(), name)
			assert.Contains(t, tool.InputSchema().Properties, property, name)
			assert.Nil(t, tool.OutputSchema(), name)
		}
	})

	t.Run("MaxResults", func(t *testing.T) {
		// The number of results is limited to 1 to 10 in the schema
		tools, _ := newTools(t)
		maxResults := tools["search"].InputSchema().Properties["max_results"]
		require.NotNil(t, maxResults)
		require.NotNil(t, maxResults.Minimum)
		require.NotNil(t, maxResults.Maximum)
		assert.Equal(t, float64(1), *maxResults.Minimum)
		assert.Equal(t, float64(10), *maxResults.Maximum)
		assert.JSONEq(t, `5`, string(maxResults.Default))
	})
}

func TestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Search", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["search"].Run(ctx, json.RawMessage(`{"query":" what is ollama? ","max_results":3}`))
		require.NoError(t, err)
		response, ok := result.(httpclient.SearchResponse)
		require.True(t, ok, "result is %T", result)
		require.Len(t, response.Results, 1)
		assert.Equal(t, "https://ollama.com", response.Results[0].URL)

		// The query is trimmed
		require.Len(t, *requests, 1)
		assert.Equal(t, "/web_search", (*requests)[0].path)
		assert.Equal(t, map[string]any{"query": "what is ollama?", "max_results": float64(3)}, (*requests)[0].body)
	})

	t.Run("SearchDefaultResults", func(t *testing.T) {
		// Without a maximum number of results, five are returned
		tools, requests := newTools(t)
		_, err := tools["search"].Run(ctx, json.RawMessage(`{"query":"ollama"}`))
		require.NoError(t, err)
		require.Len(t, *requests, 1)
		assert.Equal(t, map[string]any{"query": "ollama", "max_results": float64(5)}, (*requests)[0].body)
	})

	t.Run("Fetch", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["fetch"].Run(ctx, json.RawMessage(`{"url":"https://ollama.com"}`))
		require.NoError(t, err)
		response, ok := result.(httpclient.FetchResponse)
		require.True(t, ok, "result is %T", result)
		assert.Equal(t, "Ollama", response.Title)
		assert.Equal(t, []string{"https://ollama.com/download"}, response.Links)
		require.Len(t, *requests, 1)
		assert.Equal(t, "/web_fetch", (*requests)[0].path)
		assert.Equal(t, map[string]any{"url": "https://ollama.com"}, (*requests)[0].body)
	})

	t.Run("Invalid", func(t *testing.T) {
		tools, requests := newTools(t)
		for name, test := range map[string]struct {
			tool  string
			input string
		}{
			"MissingQuery":   {"search", `{}`},
			"EmptyQuery":     {"search", `{"query":" "}`},
			"TooManyResults": {"search", `{"query":"ollama","max_results":20}`},
			"MissingURL":     {"fetch", `{}`},
			"NoInput":        {"fetch", ``},
			"InvalidJSON":    {"search", `{`},
		} {
			result, err := tools[test.tool].Run(ctx, json.RawMessage(test.input))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
			assert.Nil(t, result, name)
		}

		// Invalid input isn't sent to Ollama
		assert.Empty(t, *requests)
	})

	t.Run("Cancelled", func(t *testing.T) {
		// The request is cancelled with the context
		tools, requests := newTools(t)
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := tools["search"].Run(ctx, json.RawMessage(`{"query":"ollama"}`))
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, *requests)
	})
}
