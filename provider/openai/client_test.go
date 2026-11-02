package openai_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

// apiKey is the key the test server accepts
const apiKey = "sk-test"

// models is the response from the test server's GET /v1/models: an OpenAI
// model, and models with the extra fields OpenAI-compatible servers add
const models = `{
	"object": "list",
	"data": [
		{"id": "gpt-4o", "object": "model", "created": 1715367049, "owned_by": "system"},
		{"id": "meta-llama/Llama-3.1-8B", "object": "model", "created": 1715367050, "owned_by": "vllm", "max_model_len": 8192},
		{"id": "openrouter/auto", "object": "model", "description": "Routes to the best model", "context_length": 128000},
		{"id": "deepseek-v4-pro", "object": "model", "owned_by": "deepseek", "name": "DeepSeek-V4-Pro", "context_window": 1048576, "max_output_tokens": 393216}
	]
}`

// newServer returns a test server for an OpenAI-compatible API, which
// requires the key, if it isn't empty
func newServer(t *testing.T, key string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key != "" && r.Header.Get("Authorization") != "Bearer "+key {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"Incorrect API key provided","type":"invalid_request_error"}}`))
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(models))
	}))
	t.Cleanup(server.Close)
	return server
}

// newClient returns a client for a test server which requires the API key
func newClient(t *testing.T) *openai.Client {
	t.Helper()
	c, err := openai.New(newServer(t, apiKey).URL+"/v1", apiKey)
	require.NoError(t, err)
	return c
}

///////////////////////////////////////////////////////////////////////////////
// New

func Test_New_DefaultEndpoint(t *testing.T) {
	c, err := openai.New("", "")
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func Test_New_HostPortNoScheme(t *testing.T) {
	// A bare host:port (no scheme) is accepted, and normalised to
	// "https://host:port/v1"
	c, err := openai.New("localhost:8000", "")
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func Test_New_URLWithNoPath(t *testing.T) {
	// A URL with no path gets "/v1" appended
	c, err := openai.New(newServer(t, "").URL, "")
	require.NoError(t, err)
	assert.NoError(t, c.Ping(context.Background()))
}

func Test_URL(t *testing.T) {
	// The endpoint is normalized
	for endpoint, want := range map[string]string{
		"":                             "https://api.openai.com/v1",
		"localhost:8000":               "https://localhost:8000/v1",
		"http://localhost:8000":        "http://localhost:8000/v1",
		"http://localhost:8000/":       "http://localhost:8000/v1",
		"http://localhost:8000/v1":     "http://localhost:8000/v1",
		"https://openrouter.ai/api/v1": "https://openrouter.ai/api/v1",
	} {
		c, err := openai.New(endpoint, "")
		require.NoError(t, err, "endpoint %q", endpoint)
		require.NotNil(t, c.URL())
		assert.Equal(t, want, c.URL().String(), "endpoint %q", endpoint)
	}
}

///////////////////////////////////////////////////////////////////////////////
// Name / Self / interface conformance

func Test_Name(t *testing.T) {
	c, err := openai.New("", "")
	require.NoError(t, err)
	assert.Equal(t, llm.OpenAI, c.Name())
}

func Test_Self(t *testing.T) {
	c, err := openai.New("", "")
	require.NoError(t, err)
	assert.Same(t, c, c.Self())
}

func Test_ImplementsProvider(t *testing.T) {
	assert.Implements(t, (*llm.Provider)(nil), &openai.Client{})
}

///////////////////////////////////////////////////////////////////////////////
// Ping

func Test_Ping(t *testing.T) {
	assert.NoError(t, newClient(t).Ping(context.Background()))
}

func Test_Ping_NoKeyRequired(t *testing.T) {
	// A server which doesn't need a key, such as a local one
	c, err := openai.New(newServer(t, "").URL+"/v1", "")
	require.NoError(t, err)
	assert.NoError(t, c.Ping(context.Background()))
}

func Test_Ping_WrongKey(t *testing.T) {
	// The key is checked, so a wrong or missing key fails to ping
	for _, key := range []string{"sk-wrong", ""} {
		c, err := openai.New(newServer(t, apiKey).URL+"/v1", key)
		require.NoError(t, err)
		assert.Error(t, c.Ping(context.Background()), "key %q", key)
	}
}

func Test_Ping_Interval(t *testing.T) {
	// A server which counts its requests
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(models))
	}))
	t.Cleanup(server.Close)
	c, err := openai.New(server.URL+"/v1", "")
	require.NoError(t, err)
	ctx := context.Background()

	// The API is pinged once within the ping interval
	for range 3 {
		require.NoError(t, c.Ping(ctx))
	}
	assert.EqualValues(t, 1, requests.Load())

	// Listing models still makes a request
	_, err = c.ListModels(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 2, requests.Load())
}

func Test_Ping_IntervalError(t *testing.T) {
	// A failed ping is returned until the ping interval has passed
	c, err := openai.New(newServer(t, apiKey).URL+"/v1", "sk-wrong")
	require.NoError(t, err)
	first := c.Ping(context.Background())
	require.Error(t, first)
	assert.Equal(t, first, c.Ping(context.Background()))
}

func Test_Ping_Unreachable(t *testing.T) {
	// A client pointed at a port nothing is listening on fails to ping
	c, err := openai.New("http://localhost:1", "")
	require.NoError(t, err)
	assert.Error(t, c.Ping(context.Background()))
}

///////////////////////////////////////////////////////////////////////////////
// ListModels

func Test_ListModels(t *testing.T) {
	models, err := newClient(t).ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 4)

	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name()
	}
	assert.Equal(t, []string{"gpt-4o", "meta-llama/Llama-3.1-8B", "openrouter/auto", "deepseek-v4-pro"}, names)
}

func Test_Model_Fields(t *testing.T) {
	c := newClient(t)
	ctx := context.Background()

	t.Run("OpenAI", func(t *testing.T) {
		m, err := c.GetModel(ctx, "gpt-4o")
		require.NoError(t, err)
		assert.Same(t, c, m.Provider())
		assert.True(t, m.Created().Equal(time.Unix(1715367049, 0)))
		assert.Empty(t, m.Description())
		assert.Nil(t, m.Aliases())
		assert.Nil(t, m.InputTokenLimit())
		assert.Nil(t, m.OutputTokenLimit())

		// Meta has the fields apart from the name and creation time
		meta := m.Meta()
		assert.JSONEq(t, `"system"`, string(meta["owned_by"]))
		assert.JSONEq(t, `"model"`, string(meta["object"]))
		assert.NotContains(t, meta, "id")
		assert.NotContains(t, meta, "created")
	})

	t.Run("MaxModelLen", func(t *testing.T) {
		m, err := c.GetModel(ctx, "meta-llama/Llama-3.1-8B")
		require.NoError(t, err)
		require.NotNil(t, m.InputTokenLimit())
		assert.Equal(t, uint(8192), *m.InputTokenLimit())
		assert.JSONEq(t, `8192`, string(m.Meta()["max_model_len"]))
	})

	t.Run("DeepSeek", func(t *testing.T) {
		// DeepSeek's context window and maximum output tokens
		m, err := c.GetModel(ctx, "deepseek-v4-pro")
		require.NoError(t, err)
		require.NotNil(t, m.InputTokenLimit())
		assert.Equal(t, uint(1048576), *m.InputTokenLimit())
		require.NotNil(t, m.OutputTokenLimit())
		assert.Equal(t, uint(393216), *m.OutputTokenLimit())

		// The display name is the description
		assert.Equal(t, "DeepSeek-V4-Pro", m.Description())
	})

	t.Run("ContextLengthAndDescription", func(t *testing.T) {
		m, err := c.GetModel(ctx, "openrouter/auto")
		require.NoError(t, err)
		require.NotNil(t, m.InputTokenLimit())
		assert.Equal(t, uint(128000), *m.InputTokenLimit())
		assert.Equal(t, "Routes to the best model", m.Description())
		assert.True(t, m.Created().IsZero(), "no created time")
	})
}

///////////////////////////////////////////////////////////////////////////////
// GetModel

func Test_GetModel_NotFound(t *testing.T) {
	_, err := newClient(t).GetModel(context.Background(), "gpt-nonexistent")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_GetModel_WrongKey(t *testing.T) {
	c, err := openai.New(newServer(t, apiKey).URL+"/v1", "sk-wrong")
	require.NoError(t, err)
	_, err = c.GetModel(context.Background(), "gpt-4o")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "Unauthorized"), "error: %v", err)
}

func Test_ListModels_ShutDown(t *testing.T) {
	// Models which have been shut down aren't listed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[
			{"id":"retired","shutdown_date":"2000-01-01"},
			{"id":"retiring","shutdown_date":"2999-01-01"},
			{"id":"current","shutdown_date":null},
			{"id":"undated"}
		]}`))
	}))
	t.Cleanup(server.Close)
	c, err := openai.New(server.URL+"/v1", "")
	require.NoError(t, err)
	ctx := context.Background()

	models, err := c.ListModels(ctx)
	require.NoError(t, err)
	var names []string
	for _, m := range models {
		names = append(names, m.Name())
	}
	assert.Equal(t, []string{"retiring", "current", "undated"}, names)

	// A model which is being retired keeps its date
	m, err := c.GetModel(ctx, "retiring")
	require.NoError(t, err)
	assert.JSONEq(t, `"2999-01-01"`, string(m.Meta()["shutdown_date"]))

	// And one which has been shut down isn't found
	_, err = c.GetModel(ctx, "retired")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
