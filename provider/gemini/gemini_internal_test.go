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
	schema "github.com/mutablelogic/go-llm/provider/schema"
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
	embedded batchEmbedRequest // The last request to embed messages
	reply    []string          // The pieces of the reply the next request to generate streams
	asked    generateRequest   // The last request to generate
}

// models are the fake's models, by page
var models = [][]apiModel{
	{
		{Name: "models/gemini-new", BaseModelID: "gemini-new", Version: "3.0", DisplayName: "Gemini New", Description: "The newest", InputTokenLimit: 1048576, OutputTokenLimit: 65536, SupportedGenerationMethods: []string{"generateContent", "countTokens"}, Temperature: types.Ptr(1.0), MaxTemperature: types.Ptr(2.0), Thinking: types.Ptr(true)},
		{Name: "models/gemini-old-001", BaseModelID: "gemini-old", DisplayName: "Gemini Old", SupportedGenerationMethods: []string{"generateContent"}, Thinking: types.Ptr(false)},
	},
	{
		{Name: "models/embedding-new", DisplayName: "Embedding", SupportedGenerationMethods: []string{"embedContent"}},
		{Name: "models/gemini-new-preview-tts", DisplayName: "Gemini New TTS", SupportedGenerationMethods: []string{"generateContent", "countTokens"}},
		{Name: "models/gemini-new-transcribe", DisplayName: "Gemini New Transcribe", SupportedGenerationMethods: []string{"generateContent", "countTokens"}},
		{Name: "models/gemini-new-image", DisplayName: "Nano Banana New", SupportedGenerationMethods: []string{"generateContent", "countTokens"}, Thinking: types.Ptr(true)},
		{Name: "models/imagen-new-generate", DisplayName: "Imagen New", SupportedGenerationMethods: []string{"predict"}},
		{Name: "models/gemini-embedding-2", DisplayName: "Gemini Embedding 2", SupportedGenerationMethods: []string{"embedContent"}},
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
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":batchEmbedContents"):
			// A vector of each message's text's length, and its parts
			var req batchEmbedRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			api.embedded = req
			var response batchEmbedResponse
			for _, embed := range req.Requests {
				response.Embeddings = append(response.Embeddings, struct {
					Values []float32 `json:"values"`
				}{Values: []float32{float32(len(embed.Content.Parts[0].Text)), float32(len(embed.Content.Parts))}})
			}
			_ = json.NewEncoder(w).Encode(response)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":streamGenerateContent"):
			// The reply, streamed, as server-sent events
			require.Equal(t, "sse", r.URL.Query().Get("alt"))
			var req generateRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			api.asked = req
			w.Header().Set("Content-Type", "text/event-stream")
			for _, piece := range api.reply {
				_, _ = w.Write([]byte("data: " + piece + "\r\n\r\n"))
			}
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
	require.Len(t, list, 8)
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

	// Models which speak, transcribe, or make images, by their names, which
	// the API doesn't say, and which don't reply, apart from those which
	// make images, as they reply with them
	assert.Equal(t, "speech", list[3].Capabilities().String())
	assert.Equal(t, "transcription", list[4].Capabilities().String())
	assert.Equal(t, "completion, thinking, image", list[5].Capabilities().String())
	assert.Equal(t, "image", list[6].Capabilities().String())
	assert.False(t, types.Value(newer.Capabilities().Image), "a model which replies doesn't make images")
	assert.False(t, types.Value(newer.Capabilities().Speech))

	// A model is got by its name, with or without the prefix, and the API is
	// pinged
	model, err := c.GetModel(ctx, "gemini-new")
	require.NoError(t, err)
	assert.Equal(t, "Gemini New", model.Description())
	model, err = c.GetModel(ctx, "models/gemini-new")
	require.NoError(t, err)
	assert.Equal(t, "gemini-new", model.Name())
	_, err = c.GetModel(ctx, "gemini-missing")
	assert.ErrorIs(t, err, llm.ErrNotFound)
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

func Test_Embed(t *testing.T) {
	ctx := context.Background()
	api := newFakeAPI(t)
	c, err := New(api.URL, "key")
	require.NoError(t, err)
	model, err := c.GetModel(ctx, "embedding-new")
	require.NoError(t, err)

	hello, err := schema.NewMessage("user", "hello")
	require.NoError(t, err)

	// A vector for each message, in order, in one request, with the model,
	// and the options, which are the same for each
	image := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("PNG")}}
	withImage := schema.Message{MessageMeta: schema.MessageMeta{Sender: "user", Content: []schema.ContentBlock{{Text: types.Ptr("a cat")}, {Resource: image}}}}
	vectors, _, err := c.Embed(ctx, model, []llm.Message{hello, &withImage},
		llm.WithTaskType("retrieval_document"), llm.WithTitle("Pets"), llm.WithDimensions(256))
	require.NoError(t, err)
	assert.Equal(t, []llm.Vector{{5, 1}, {5, 2}}, vectors)
	require.Len(t, api.embedded.Requests, 2)
	first, second := api.embedded.Requests[0], api.embedded.Requests[1]
	assert.Equal(t, "models/embedding-new", first.Model)
	assert.Equal(t, "RETRIEVAL_DOCUMENT", first.TaskType)
	assert.Equal(t, "Pets", first.Title)
	assert.Equal(t, uint(256), first.OutputDimensionality)
	assert.Equal(t, "RETRIEVAL_DOCUMENT", second.TaskType, "each message has the options")

	// An attachment is inline, with its type, and its data
	require.Len(t, second.Content.Parts, 2)
	assert.Equal(t, "image/png", second.Content.Parts[1].InlineData.MimeType)
	assert.Equal(t, []byte("PNG"), second.Content.Parts[1].InlineData.Data)

	// A task type which isn't one, an option which isn't for embeddings, no
	// messages, and a message without text, or attachments, aren't embedded
	_, _, err = c.Embed(ctx, model, []llm.Message{hello}, llm.WithTaskType("summarizing"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.ErrorContains(t, err, "retrieval_query")
	_, _, err = c.Embed(ctx, model, []llm.Message{hello}, llm.WithOpt(llm.TemperatureKey, 0.5))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Embed(ctx, model, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Embed(ctx, model, []llm.Message{&schema.Message{MessageMeta: schema.MessageMeta{Sender: "user"}}})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_EmbedInText(t *testing.T) {
	// gemini-embedding-2 takes the task type, and the title, in the text,
	// with Google's prefixes, rather than as fields
	ctx := context.Background()
	api := newFakeAPI(t)
	c, err := New(api.URL, "key")
	require.NoError(t, err)
	model, err := c.GetModel(ctx, "gemini-embedding-2")
	require.NoError(t, err)
	hello, err := schema.NewMessage("user", "hello")
	require.NoError(t, err)
	text := func(opts ...llm.Opt) string {
		t.Helper()
		_, _, err := c.Embed(ctx, model, []llm.Message{hello}, opts...)
		require.NoError(t, err)
		request := api.embedded.Requests[0]
		assert.Empty(t, request.TaskType, "no task type field")
		assert.Empty(t, request.Title, "no title field")
		return request.Content.Parts[0].Text
	}
	assert.Equal(t, "hello", text())
	assert.Equal(t, "task: search result | query: hello", text(llm.WithTaskType("retrieval_query")))
	assert.Equal(t, "title: none | text: hello", text(llm.WithTaskType("retrieval_document")))
	assert.Equal(t, "title: Pets | text: hello", text(llm.WithTaskType("retrieval_document"), llm.WithTitle("Pets")))
	assert.Equal(t, "title: Pets | text: hello", text(llm.WithTitle("Pets")))
	assert.Equal(t, "task: question answering | query: hello", text(llm.WithTaskType("question_answering")))
	assert.Equal(t, "task: fact checking | query: hello", text(llm.WithTaskType("fact_verification")))
	assert.Equal(t, "task: code retrieval | query: hello", text(llm.WithTaskType("code_retrieval_query")))
	assert.Equal(t, "task: sentence similarity | query: hello", text(llm.WithTaskType("semantic_similarity")))
	assert.Equal(t, "task: classification | query: hello", text(llm.WithTaskType("classification")))
	assert.Equal(t, "task: clustering | query: hello", text(llm.WithTaskType("clustering")))

	// The text of a message with attachments isn't prefixed, as Google
	// advises
	image := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("PNG")}}
	withImage := schema.Message{MessageMeta: schema.MessageMeta{Sender: "user", Content: []schema.ContentBlock{{Text: types.Ptr("a cat")}, {Resource: image}}}}
	_, _, err = c.Embed(ctx, model, []llm.Message{&withImage}, llm.WithTaskType("retrieval_query"))
	require.NoError(t, err)
	assert.Equal(t, "a cat", api.embedded.Requests[0].Content.Parts[0].Text)

	// A title is only for a document, with any model
	_, _, err = c.Embed(ctx, model, []llm.Message{hello}, llm.WithTaskType("retrieval_query"), llm.WithTitle("Pets"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	older, err := c.GetModel(ctx, "embedding-new")
	require.NoError(t, err)
	_, _, err = c.Embed(ctx, older, []llm.Message{hello}, llm.WithTaskType("classification"), llm.WithTitle("Pets"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
