package deepseek_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	deepseek "github.com/mutablelogic/go-llm/toolkit/deepseek"
	httpclient "github.com/mutablelogic/go-llm/toolkit/deepseek/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// request is a request received by the fake DeepSeek server
type request struct {
	path string
	body map[string]any
}

// searchResponse is a response as DeepSeek documents it, with the fields
// which aren't returned to the model, and a result without a site or date
const searchResponse = `{"results":[
	{"rerank_score":2.4,"title":"DeepSeek API Docs","url":"https://api-docs.deepseek.com/","snippet":"Short.","summary":"The DeepSeek API is compatible with OpenAI's.","content":"","site_name":"DeepSeek","site_icon":"https://api-docs.deepseek.com/favicon.ico","date":"2026-10-17T08:00:00Z","authority":3},
	{"rerank_score":1.1,"title":"Elsewhere","url":"https://example.com/","snippet":"","summary":"Another page.","content":"","site_name":null,"site_icon":null,"date":null,"authority":-1}
]}`

// newTools returns the tools, with a client for a fake DeepSeek server, and
// the requests the server received
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

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search":
			w.Write([]byte(searchResponse))
		case "/fetch":
			if body["url"] == "https://example.com/gone" {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":{"message":"No usable content","type":"invalid_request_error","param":null,"code":"not_found"}}`))
				return
			}
			w.Write([]byte(`{"title":"Your First API Call | DeepSeek API Docs","content":"# Your First API Call\n\nThe DeepSeek API uses an API format compatible with OpenAI."}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	tools, err := toolsOf(deepseek.New("test-key", client.OptEndpoint(server.URL)))
	require.NoError(t, err)
	result := make(map[string]llm.Tool, len(tools))
	for _, tool := range tools {
		result[tool.Name()] = tool
	}
	return result, &requests
}

func TestNew(t *testing.T) {
	t.Run("MissingKey", func(t *testing.T) {
		tools, err := toolsOf(deepseek.New(""))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.Nil(t, tools)
	})

	t.Run("Tools", func(t *testing.T) {
		tools, _ := newTools(t)
		assert.Len(t, tools, 2)
		for name, property := range map[string]string{"search": "query", "fetch": "url"} {
			tool, ok := tools[name]
			require.True(t, ok, name)
			assert.Contains(t, tool.Description(), "billed", name)
			assert.Contains(t, tool.InputSchema().Properties, property, name)
			assert.Nil(t, tool.OutputSchema(), name)
		}
		assert.Contains(t, tools["search"].InputSchema().Properties, "ori_question")
		assert.Contains(t, tools["search"].InputSchema().Properties["query"].Description, "site:")
	})

	t.Run("Count", func(t *testing.T) {
		// The number of results is limited to 1 to 10 in the schema
		tools, _ := newTools(t)
		count := tools["search"].InputSchema().Properties["count"]
		require.NotNil(t, count)
		require.NotNil(t, count.Minimum)
		require.NotNil(t, count.Maximum)
		assert.Equal(t, float64(1), *count.Minimum)
		assert.Equal(t, float64(10), *count.Maximum)
		assert.JSONEq(t, `3`, string(count.Default))
	})
}

func TestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Search", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["search"].Run(ctx, json.RawMessage(`{"query":" deepseek api ","count":3,"ori_question":" How do I call DeepSeek? "}`))
		require.NoError(t, err)
		response, ok := result.(httpclient.SearchResponse)
		require.True(t, ok, "result is %T", result)
		require.Len(t, response.Results, 2)

		// A result has its title, URL, site, date and summary
		first := response.Results[0]
		assert.Equal(t, "https://api-docs.deepseek.com/", first.URL)
		assert.Equal(t, "The DeepSeek API is compatible with OpenAI's.", first.Summary)
		require.NotNil(t, first.SiteName)
		assert.Equal(t, "DeepSeek", *first.SiteName)
		require.NotNil(t, first.Date)
		assert.True(t, first.Date.Equal(time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)))
		assert.Nil(t, response.Results[1].SiteName)
		assert.Nil(t, response.Results[1].Date)

		// And only those, as the model reads them
		data, err := json.Marshal(response)
		require.NoError(t, err)
		for _, field := range []string{"rerank_score", "snippet", "site_icon", "authority", `"content"`} {
			assert.NotContains(t, string(data), field)
		}
		assert.NotContains(t, string(data[strings.Index(string(data), "Elsewhere"):]), "site_name", "a result without a site has none")

		// The query and question are trimmed
		require.Len(t, *requests, 1)
		assert.Equal(t, "/search", (*requests)[0].path)
		assert.Equal(t, map[string]any{"query": "deepseek api", "count": float64(3), "ori_question": "How do I call DeepSeek?"}, (*requests)[0].body)
	})

	t.Run("SearchDefaults", func(t *testing.T) {
		// Without a count, three results, and without a question, or an empty
		// one, none is sent
		tools, requests := newTools(t)
		for _, input := range []string{`{"query":"deepseek"}`, `{"query":"deepseek","ori_question":" "}`} {
			_, err := tools["search"].Run(ctx, json.RawMessage(input))
			require.NoError(t, err)
		}
		require.Len(t, *requests, 2)
		for _, request := range *requests {
			assert.Equal(t, map[string]any{"query": "deepseek", "count": float64(3)}, request.body)
		}
	})

	t.Run("Fetch", func(t *testing.T) {
		tools, requests := newTools(t)
		result, err := tools["fetch"].Run(ctx, json.RawMessage(`{"url":" https://api-docs.deepseek.com/ "}`))
		require.NoError(t, err)
		response, ok := result.(httpclient.FetchResponse)
		require.True(t, ok, "result is %T", result)
		assert.Equal(t, "Your First API Call | DeepSeek API Docs", response.Title)
		assert.True(t, strings.HasPrefix(response.Content, "# Your First API Call"))
		require.Len(t, *requests, 1)
		assert.Equal(t, "/fetch", (*requests)[0].path)
		assert.Equal(t, map[string]any{"url": "https://api-docs.deepseek.com/"}, (*requests)[0].body)
	})

	t.Run("FetchFails", func(t *testing.T) {
		// A page DeepSeek can't read is an error, which says why
		tools, _ := newTools(t)
		result, err := tools["fetch"].Run(ctx, json.RawMessage(`{"url":"https://example.com/gone"}`))
		require.Error(t, err)
		assert.ErrorContains(t, err, "No usable content")
		assert.Nil(t, result)
	})

	t.Run("Invalid", func(t *testing.T) {
		tools, requests := newTools(t)
		for name, test := range map[string]struct {
			tool  string
			input string
		}{
			"MissingQuery":    {"search", `{}`},
			"EmptyQuery":      {"search", `{"query":" "}`},
			"LongQuery":       {"search", `{"query":"` + strings.Repeat("ß", 129) + `"}`},
			"LongQuestion":    {"search", `{"query":"deepseek","ori_question":"` + strings.Repeat("x", 129) + `"}`},
			"TooManyResults":  {"search", `{"query":"deepseek","count":20}`},
			"NegativeResults": {"search", `{"query":"deepseek","count":-1}`},
			"MissingURL":      {"fetch", `{}`},
			"NoInput":         {"fetch", ``},
			"NotHTTP":         {"fetch", `{"url":"file:///etc/passwd"}`},
			"NoHost":          {"fetch", `{"url":"https:///path"}`},
			"Password":        {"fetch", `{"url":"https://user:secret@example.com/"}`},
			"LongURL":         {"fetch", `{"url":"https://example.com/` + strings.Repeat("a", 8192) + `"}`},
			"InvalidJSON":     {"search", `{`},
		} {
			result, err := tools[test.tool].Run(ctx, json.RawMessage(test.input))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
			assert.Nil(t, result, name)
		}

		// A query of 128 characters, and a URL of 8,192, of more bytes, are
		// allowed, and one character more isn't
		_, err := tools["search"].Run(ctx, json.RawMessage(`{"query":"`+strings.Repeat("ß", 128)+`"}`))
		require.NoError(t, err)
		prefix := "https://example.com/"
		path := strings.Repeat("页", 8192-len(prefix))
		_, err = tools["fetch"].Run(ctx, json.RawMessage(`{"url":"`+prefix+path+`"}`))
		require.NoError(t, err)
		_, err = tools["fetch"].Run(ctx, json.RawMessage(`{"url":"`+prefix+path+`页"}`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)

		// Invalid input isn't sent to DeepSeek, so it isn't billed
		assert.Len(t, *requests, 2)
	})

	t.Run("Cancelled", func(t *testing.T) {
		// The request is cancelled with the context
		tools, requests := newTools(t)
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := tools["search"].Run(ctx, json.RawMessage(`{"query":"deepseek"}`))
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, *requests)
	})
}

// toolsOf returns the tools of the namespace, or nil and the error
func toolsOf(ns toolkit.Namespace, err error) ([]llm.Tool, error) {
	if err != nil {
		return nil, err
	}
	return ns.Tools(), nil
}
