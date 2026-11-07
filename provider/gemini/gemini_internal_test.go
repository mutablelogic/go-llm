package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// fakeAPI is a fake of the Gemini API, which lists two models, on two
// pages, and an embedding model, gets them, and counts the tokens of text
// as its bytes, and three more, failing as many counts as it's told to
type fakeAPI struct {
	*httptest.Server
	counts   atomic.Int32
	failures atomic.Int32
}

// models are the fake's models, by page
var models = [][]apiModel{
	{
		{Name: "models/gemini-new", BaseModelID: "gemini-new", Version: "3.0", DisplayName: "Gemini New", Description: "The newest", InputTokenLimit: 1048576, OutputTokenLimit: 65536, SupportedGenerationMethods: []string{"generateContent", "countTokens"}, Temperature: types.Ptr(1.0), MaxTemperature: types.Ptr(2.0), Thinking: types.Ptr(true)},
		{Name: "models/gemini-old-001", BaseModelID: "gemini-old", DisplayName: "Gemini Old", SupportedGenerationMethods: []string{"generateContent"}, Thinking: types.Ptr(false)},
	},
	{
		{Name: "models/embedding-new", DisplayName: "Embedding", SupportedGenerationMethods: []string{"embedContent"}},
	},
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	api := &fakeAPI{}
	api.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("x-goog-api-key") != "key" || r.URL.Query().Get("key") != "" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":403,"message":"API key not valid","status":"PERMISSION_DENIED"}}`))
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1beta/models":
			page := 0
			if r.URL.Query().Get("pageToken") == "two" {
				page = 1
			}
			list := modelList{Models: models[page]}
			if page == 0 {
				list.NextPageToken = "two"
			}
			_ = json.NewEncoder(w).Encode(list)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1beta/models/"):
			name := strings.TrimPrefix(r.URL.Path, "/v1beta/")
			for _, page := range models {
				for _, model := range page {
					if model.Name == name {
						_ = json.NewEncoder(w).Encode(model)
						return
					}
				}
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"not found","status":"NOT_FOUND"}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":countTokens"):
			if api.failures.Load() > 0 {
				api.failures.Add(-1)
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"code":429,"message":"rate limited","status":"RESOURCE_EXHAUSTED"}}`))
				return
			}
			api.counts.Add(1)
			var req countRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			_ = json.NewEncoder(w).Encode(countResponse{TotalTokens: uint(len(req.Contents[0].Parts[0].Text)) + 3})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(api.Close)
	return api
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_New(t *testing.T) {
	_, err := New("", "")
	assert.ErrorIs(t, err, llm.ErrBadParameter, "it needs an API key")

	c, err := New("", "key")
	require.NoError(t, err)
	assert.Equal(t, Endpoint, c.URL().String())
	assert.Equal(t, llm.Gemini, c.Name())

	c, err = New("example.com", "key")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1beta", c.URL().String())
}

func Test_Models(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	c, err := New(api.URL, "key")
	require.NoError(t, err)

	// Every page of the models is listed, by their names, without the API's
	// prefix, with their limits, and what they can do
	list, err := c.ListModels(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	newer, older, embedding := list[0], list[1], list[2]
	assert.Equal(t, "gemini-new", newer.Name())
	assert.Equal(t, "Gemini New", newer.Description())
	assert.Equal(t, uint(1048576), types.Value(newer.InputTokenLimit()))
	assert.Equal(t, uint(65536), types.Value(newer.OutputTokenLimit()))
	assert.Equal(t, "completion, thinking", newer.Capabilities().String())
	assert.True(t, types.Value(newer.Capabilities().Temperature))
	assert.Nil(t, newer.Capabilities().Tools, "whether it calls tools isn't known")
	assert.Empty(t, newer.Aliases())
	assert.JSONEq(t, `"3.0"`, string(newer.Meta()["version"]))
	assert.JSONEq(t, `["generateContent","countTokens"]`, string(newer.Meta()["supported_generation_methods"]))
	assert.Equal(t, "gemini-old-001", older.Name())
	assert.Equal(t, []string{"gemini-old"}, older.Aliases())
	assert.Equal(t, "completion", older.Capabilities().String())
	assert.Equal(t, "embedding", embedding.Capabilities().String())
	assert.False(t, types.Value(embedding.Capabilities().Thinking))

	// A model is got by its name, with or without the prefix, and the API is
	// pinged
	model, err := c.GetModel(ctx, "gemini-new")
	require.NoError(t, err)
	assert.Equal(t, "Gemini New", model.Description())
	model, err = c.GetModel(ctx, "models/gemini-new")
	require.NoError(t, err)
	assert.Equal(t, "gemini-new", model.Name())
	_, err = c.GetModel(ctx, "gemini-missing")
	assert.Error(t, err)
	_, err = c.GetModel(ctx, " ")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.NoError(t, c.Ping(ctx))

	// A key which isn't accepted fails
	bad, err := New(api.URL, "wrong")
	require.NoError(t, err)
	assert.Error(t, bad.Ping(ctx))
}

func Test_Measure(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	c, err := New(api.URL, "key")
	require.NoError(t, err)
	model, err := c.GetModel(ctx, "gemini-new")
	require.NoError(t, err)

	// The tokens of the text are those of the message, less those of a
	// message with one token, apart from that token, which are counted once,
	// and text which was counted isn't counted again
	tokens, err := c.Measure(ctx, model, "hello")
	require.NoError(t, err)
	assert.Equal(t, uint(5), tokens)
	assert.Equal(t, int32(2), api.counts.Load())
	tokens, err = c.Measure(ctx, model, "hello")
	require.NoError(t, err)
	assert.Equal(t, uint(5), tokens)
	assert.Equal(t, int32(2), api.counts.Load(), "it isn't counted again")
	tokens, err = c.Measure(ctx, model, "")
	require.NoError(t, err)
	assert.Zero(t, tokens)

	// When tokens can't be counted, such as when counting is rate limited,
	// they're an estimate
	api.failures.Store(1)
	tokens, err = c.Measure(ctx, model, "twelve bytes")
	require.NoError(t, err)
	assert.Equal(t, uint(3), tokens)

	// Unless the context is done
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.Measure(cancelled, model, "something else")
	assert.Error(t, err)

	// A model of another provider isn't measured
	other, err := New(api.URL, "key")
	require.NoError(t, err)
	_, err = other.Measure(ctx, model, "hello")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
