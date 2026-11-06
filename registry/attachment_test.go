package registry_test

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	resourcemanager "github.com/mutablelogic/go-llm/resource"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// pngData is the start of a PNG image
const pngData = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

// attached returns a message from the user, with the text, and an
// attachment which refers to the resource with the URI
func attached(t *testing.T, text, uri string) *providerschema.Message {
	t.Helper()
	u, err := url.Parse(uri)
	require.NoError(t, err)
	return &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{
		{Text: types.Ptr(text)},
		{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{URL: u}}},
	}}}
}

// attachments returns the attachments of the message, as their types and
// content
func attachments(t *testing.T, message llm.Message) []string {
	t.Helper()
	var result []string
	for _, r := range message.Resources() {
		data, err := llm.ReadAll(context.Background(), r)
		require.NoError(t, err)
		result = append(result, r.Type()+":"+string(data))
	}
	return result
}

func Test_Attachments(t *testing.T) {
	ctx := context.Background()
	sessions := test.NewSessionStore()
	manager, err := resourcemanager.New(resourcemanager.NewStore(), resourcemanager.WithSessions(func(ctx context.Context, id uuid.UUID) error {
		_, err := sessions.Get(ctx, id)
		return err
	}))
	require.NoError(t, err)
	p := newScripted(text("A cat"), text("Still a cat"), text("A dog"), text("Gone"), text("Spare"))
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithResources(manager))
	require.NoError(t, err)
	require.NoError(t, r.Add("", p))

	alone, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Name: "cat.png"}, strings.NewReader(pngData))
	require.NoError(t, err)

	t.Run("Ask", func(t *testing.T) {
		// An attachment which refers to a resource on its own is sent with
		// its content, and type
		p.asked = nil
		_, err := r.Ask(ctx, schema.AskRequest{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}, Messages: []providerschema.MessageMeta{attached(t, "What's this?", alone.URI()).MessageMeta}}, nil)
		require.NoError(t, err)
		require.Len(t, p.asked, 1)
		assert.Equal(t, []string{"image/png:" + pngData}, attachments(t, p.asked[0].messages[0]))

		// One which isn't a resource isn't
		p.replies = append([]*providerschema.Message{text("A cat")}, p.replies...)
		_, err = r.Ask(ctx, schema.AskRequest{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}, Messages: []providerschema.MessageMeta{attached(t, "What's this?", resourceschema.URI(strings.Repeat("0", 64))).MessageMeta}}, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		p.replies = p.replies[1:]
	})

	t.Run("Chat", func(t *testing.T) {
		// A resource on its own, which is attached in a chat, is linked to
		// its session, and the session keeps its URI, rather than its
		// content, which is read again with each request
		session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
		require.NoError(t, err)
		p.asked = nil
		_, err = r.Chat(ctx, session.ID, []llm.Message{attached(t, "What's this?", alone.URI())}, nil)
		require.NoError(t, err)
		require.Len(t, p.asked, 1)
		assert.Equal(t, []string{"image/png:" + pngData}, attachments(t, p.asked[0].messages[0]))
		assert.Contains(t, p.asked[0].messages[0].Text(), alone.URI(), "the model is told its URI, for tools")
		_, err = manager.Get(ctx, resourceschema.ResourceKey{Hash: alone.Hash, Session: &session.ID})
		require.NoError(t, err, "it's linked to the session")
		messages, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
		require.NoError(t, err)
		stored := messages.Body[0].Content[1].Resource
		require.NotNil(t, stored)
		assert.Equal(t, alone.URI(), stored.URL.String())
		assert.Empty(t, stored.Data, "the session keeps its URI")
		assert.Equal(t, "image/png", stored.ContentType)

		// It's read with the next request, from the session's resource, even
		// once the one on its own is deleted
		_, err = manager.Delete(ctx, alone.Key())
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Are you sure?")}, nil)
		require.NoError(t, err)
		require.Len(t, p.asked, 2)
		assert.Equal(t, []string{"image/png:" + pngData}, attachments(t, p.asked[1].messages[0]))

		// Once the session's is deleted too, it's text which says so
		_, err = manager.DeleteSession(ctx, session.ID)
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "And now?")}, nil)
		require.NoError(t, err)
		require.Len(t, p.asked, 3)
		assert.Empty(t, p.asked[2].messages[0].Resources())
		assert.Contains(t, p.asked[2].messages[0].Text(), "isn't available")

		// A resource of another session isn't found
		other, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
		require.NoError(t, err)
		theirs, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Session: &other.ID}, strings.NewReader("theirs"))
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{attached(t, "And this?", theirs.URI())}, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Text", func(t *testing.T) {
		// A resource which is text is sent as text, with its name, as most
		// models don't accept text as an attachment
		notes, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Name: "notes.txt", Type: "text/plain"}, strings.NewReader("The secret word is pineapple.\n"))
		require.NoError(t, err)
		p.replies = append([]*providerschema.Message{text("Pineapple")}, p.replies...)
		p.asked = nil
		_, err = r.Ask(ctx, schema.AskRequest{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}, Messages: []providerschema.MessageMeta{attached(t, "What's the word?", notes.URI()).MessageMeta}}, nil)
		require.NoError(t, err)
		require.Len(t, p.asked, 1)
		assert.Empty(t, p.asked[0].messages[0].Resources())
		assert.Contains(t, p.asked[0].messages[0].Text(), "notes.txt:\n\n```\nThe secret word is pineapple.\n```")
	})

	t.Run("Requests", func(t *testing.T) {
		// A request's resource, by its URI, is a resource on its own, whose
		// type is the resource's, and it has data, or a URL, but not both
		audio, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Type: "audio/mpeg"}, strings.NewReader("ID3"))
		require.NoError(t, err)
		req := schema.TranscribeRequest{URL: audio.URI()}
		got, err := req.Resource()
		require.NoError(t, err)
		assert.Equal(t, audio.URI(), got.URI())
		_, err = schema.TranscribeRequest{URL: audio.URI(), Data: []byte("x")}.Resource()
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = schema.RecognizeRequest{Data: []byte("x")}.Resource()
		assert.ErrorIs(t, err, llm.ErrBadParameter, "data needs a type")
		_, err = schema.RecognizeRequest{URL: "https://example.com/a.pdf"}.Resource()
		assert.ErrorIs(t, err, llm.ErrBadParameter, "only a resource's URI")
	})

	t.Run("Embed", func(t *testing.T) {
		// A resource which isn't found isn't embedded, as if it were text
		// which says so, but is an error
		embedder := newCapable("embedder", "embed")
		r, err := registry.New(registry.WithResources(manager))
		require.NoError(t, err)
		require.NoError(t, r.Add("", embedder))
		model, err := r.GetModel(ctx, "embed")
		require.NoError(t, err)
		_, _, err = r.Embed(ctx, model, []llm.Message{attached(t, "", resourceschema.URI(strings.Repeat("0", 64)))})
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Size", func(t *testing.T) {
		// The resources a request's attachments refer to are read once, and
		// at most MaxAttachmentsSize of them, together
		limit := registry.MaxAttachmentsSize
		registry.MaxAttachmentsSize = 20
		t.Cleanup(func() { registry.MaxAttachmentsSize = limit })
		ten, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Type: "image/png"}, strings.NewReader("0123456789"))
		require.NoError(t, err)
		other, _, err := manager.Create(ctx, resourceschema.ResourceMeta{Type: "image/png"}, strings.NewReader("abcdefghijk"))
		require.NoError(t, err)
		p.replies = append([]*providerschema.Message{text("Two")}, p.replies...)
		p.asked = nil
		message := attached(t, "What are these?", ten.URI())
		message.Content = append(message.Content, message.Content[1], message.Content[1])
		_, err = r.Ask(ctx, schema.AskRequest{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}, Messages: []providerschema.MessageMeta{message.MessageMeta}}, nil)
		require.NoError(t, err, "the same resource three times is read once")
		assert.Len(t, attachments(t, p.asked[0].messages[0]), 3)
		message.Content = append(message.Content, attached(t, "", other.URI()).Content[1])
		_, err = r.Ask(ctx, schema.AskRequest{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}, Messages: []providerschema.MessageMeta{message.MessageMeta}}, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter, "two resources are larger than the most, together")
	})

	t.Run("Without", func(t *testing.T) {
		// A registry without resources can't read them
		bare, err := registry.New()
		require.NoError(t, err)
		require.NoError(t, bare.Add("", newScripted(text("Hi"))))
		_, err = bare.Ask(ctx, schema.AskRequest{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}, Messages: []providerschema.MessageMeta{attached(t, "What's this?", alone.URI()).MessageMeta}}, nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
	})
}

// snapshotTool is a tool which returns an image, as a camera would
type snapshotTool struct{ toolkit.Base }

func (snapshotTool) Name() string                    { return "snapshot" }
func (snapshotTool) Description() string             { return "Takes a picture" }
func (snapshotTool) InputSchema() *jsonschema.Schema { return nil }
func (snapshotTool) Run(context.Context, json.RawMessage) (any, error) {
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/png", Data: []byte(pngData)}}, nil
}

func Test_ToolResources(t *testing.T) {
	ctx := context.Background()
	sessions := test.NewSessionStore()
	manager, err := resourcemanager.New(resourcemanager.NewStore(), resourcemanager.WithSessions(func(ctx context.Context, id uuid.UUID) error {
		_, err := sessions.Get(ctx, id)
		return err
	}))
	require.NoError(t, err)
	tk := test.NewToolkit(t)
	require.NoError(t, tk.Add(toolkit.NewNamespace("camera", "", snapshotTool{})))
	p := newScripted(call("camera__snapshot", `{}`), text("A cat"))
	r, err := registry.New(registry.WithSessionStore(sessions), registry.WithToolkit(tk), registry.WithResources(manager))
	require.NoError(t, err)
	require.NoError(t, r.Add("", p))

	// An image which a tool returns, in a chat, is kept as a resource of the
	// session, and its result is a link to it, rather than its data
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Tools: []string{"camera:*"}}})
	require.NoError(t, err)
	response, err := r.Chat(ctx, session.ID, []llm.Message{user(t, "What's at the door?")}, nil)
	require.NoError(t, err)
	var result *providerschema.ToolResult
	for _, message := range response.Messages {
		for _, block := range message.Content {
			if block.ToolResult != nil {
				result = block.ToolResult
			}
		}
	}
	require.NotNil(t, result)
	var links toolschema.ResourcesResult
	require.NoError(t, json.Unmarshal(result.Content, &links))
	require.Len(t, links.Resources, 1)
	link := links.Resources[0]
	assert.Equal(t, "image/png", link.Type)
	assert.Equal(t, int64(len(pngData)), link.Size)
	assert.NotContains(t, string(result.Content), "data", "the result has a link, rather than the data")
	got, err := manager.Resource(ctx, link.URI, &session.ID)
	require.NoError(t, err, "it's the session's")
	data, err := llm.ReadAll(ctx, got)
	require.NoError(t, err)
	assert.Equal(t, pngData, string(data))

}
