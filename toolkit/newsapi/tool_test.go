package newsapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	newsapi "github.com/mutablelogic/go-llm/toolkit/newsapi"
	httpclient "github.com/mutablelogic/go-llm/toolkit/newsapi/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// request is a request received by the fake NewsAPI server
type request struct {
	path  string
	query url.Values
}

// newTools returns the tools, with a client for a fake NewsAPI server, and the
// requests the server received
func newTools(t *testing.T) (map[string]llm.Tool, *[]request) {
	t.Helper()
	var requests []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"status":"error","code":"apiKeyInvalid","message":"Your API key is invalid."}`))
			return
		}
		requests = append(requests, request{path: r.URL.Path, query: r.URL.Query()})

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Query().Get("q") == "error":
			// An error, which NewsAPI returns in the body with its status
			w.Write([]byte(`{"status":"error","code":"parametersMissing","message":"Required parameters are missing."}`))
		case r.URL.Path == "/top-headlines" || r.URL.Path == "/everything":
			w.Write([]byte(`{"status":"ok","totalResults":1,"articles":[{"source":{"id":"bbc-news","name":"BBC News"},"title":"Headline","url":"https://bbc.co.uk/news/1","publishedAt":"2026-10-12T09:00:00Z"}]}`))
		case r.URL.Path == "/top-headlines/sources":
			w.Write([]byte(`{"status":"ok","sources":[{"id":"bbc-news","name":"BBC News","category":"general","language":"en","country":"gb"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	tools, err := newsapi.NewTools("test-key", client.OptEndpoint(server.URL))
	require.NoError(t, err)
	result := make(map[string]llm.Tool, len(tools))
	for _, tool := range tools {
		result[tool.Name()] = tool
	}
	return result, &requests
}

func TestNewTools(t *testing.T) {
	t.Run("MissingKey", func(t *testing.T) {
		tools, err := newsapi.NewTools("")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.Nil(t, tools)
	})

	t.Run("Tools", func(t *testing.T) {
		tools, _ := newTools(t)
		assert.Len(t, tools, 3)
		for _, name := range []string{"articles", "headlines", "sources"} {
			tool, ok := tools[name]
			require.True(t, ok, name)
			assert.NotEmpty(t, tool.Description(), name)
			assert.NotNil(t, tool.InputSchema(), name)
		}
	})

	t.Run("Schema", func(t *testing.T) {
		// The schemas have the allowed values and limits
		tools, _ := newTools(t)
		sortBy := tools["articles"].InputSchema().Properties["sortBy"]
		require.NotNil(t, sortBy)
		assert.Equal(t, []any{"relevancy", "popularity", "publishedAt"}, sortBy.Enum)
		for _, name := range []string{"headlines", "sources"} {
			category := tools[name].InputSchema().Properties["category"]
			require.NotNil(t, category, name)
			assert.Contains(t, category.Enum, "technology", name)
		}
		pageSize := tools["articles"].InputSchema().Properties["pageSize"]
		require.NotNil(t, pageSize)
		require.NotNil(t, pageSize.Maximum)
		assert.Equal(t, float64(100), *pageSize.Maximum)
	})
}

func TestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Articles", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["articles"].Run(ctx, json.RawMessage(`{"q":"google","sortBy":"popularity","pageSize":10}`))
		require.NoError(t, err)
		articles, ok := result.([]httpclient.Article)
		require.True(t, ok, "result is %T", result)
		require.Len(t, articles, 1)
		assert.Equal(t, "BBC News", articles[0].Source.Name)
		require.Len(t, *requests, 1)
		assert.Equal(t, "/everything", (*requests)[0].path)
		assert.Equal(t, url.Values{"q": {"google"}, "sortBy": {"popularity"}, "pageSize": {"10"}}, (*requests)[0].query)
	})

	t.Run("Headlines", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["headlines"].Run(ctx, json.RawMessage(`{"country":"gb","category":"technology"}`))
		require.NoError(t, err)
		articles, ok := result.([]httpclient.Article)
		require.True(t, ok, "result is %T", result)
		assert.Len(t, articles, 1)
		assert.Equal(t, "/top-headlines", (*requests)[0].path)
		assert.Equal(t, url.Values{"country": {"gb"}, "category": {"technology"}}, (*requests)[0].query)
	})

	t.Run("Sources", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["sources"].Run(ctx, json.RawMessage(`{"language":"en"}`))
		require.NoError(t, err)
		sources, ok := result.([]httpclient.Source)
		require.True(t, ok, "result is %T", result)
		require.Len(t, sources, 1)
		assert.Equal(t, "bbc-news", sources[0].Id)
		assert.Equal(t, "/top-headlines/sources", (*requests)[0].path)
	})

	t.Run("Error", func(t *testing.T) {
		// An error from NewsAPI is returned
		tools, _ := newTools(t)
		_, err := tools["articles"].Run(ctx, json.RawMessage(`{"q":"error"}`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.ErrorContains(t, err, "parametersMissing")
	})

	t.Run("Unauthorized", func(t *testing.T) {
		// A request with an invalid API key fails
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"status":"error","code":"apiKeyInvalid","message":"Your API key is invalid."}`))
		}))
		defer server.Close()
		tools, err := newsapi.NewTools("wrong-key", client.OptEndpoint(server.URL))
		require.NoError(t, err)
		_, err = tools[0].Run(ctx, json.RawMessage(`{"q":"google"}`))
		assert.Error(t, err)
	})

	t.Run("Invalid", func(t *testing.T) {
		tools, requests := newTools(t)
		for name, test := range map[string]struct {
			tool  string
			input string
		}{
			"SourcesAndCountry":  {"headlines", `{"sources":"bbc-news","country":"gb"}`},
			"SourcesAndCategory": {"headlines", `{"sources":"bbc-news","category":"sports"}`},
			"InvalidJSON":        {"articles", `{`},
		} {
			result, err := tools[test.tool].Run(ctx, json.RawMessage(test.input))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
			assert.Nil(t, result, name)
		}

		// Invalid input isn't sent to NewsAPI
		assert.Empty(t, *requests)
	})

	t.Run("Cancelled", func(t *testing.T) {
		tools, requests := newTools(t)
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := tools["headlines"].Run(ctx, json.RawMessage(`{"country":"gb"}`))
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, *requests)
	})
}
