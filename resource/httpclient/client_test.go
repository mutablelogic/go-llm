package httpclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	test "github.com/mutablelogic/go-llm/registry/test"
	resource "github.com/mutablelogic/go-llm/resource"
	httpclient "github.com/mutablelogic/go-llm/resource/httpclient"
	httphandler "github.com/mutablelogic/go-llm/resource/httphandler"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Client(t *testing.T) {
	ctx := t.Context()
	session := uuid.New()
	exists := func(_ context.Context, id uuid.UUID) error {
		if id == session {
			return nil
		}
		return llm.ErrNotFound.Withf("session %q", id)
	}
	r, err := resource.New(resource.NewStore(), resource.WithSessions(exists), resource.WithMaxSize(1024))
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router)
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)

	// Uploaded, as a file, whose name, and type, are the file's, on its own,
	// and in a session
	notes, err := c.CreateResource(ctx, schema.ResourceCreateRequest{Description: "Notes"}, "/tmp/notes.json", strings.NewReader(`{"a":1}`))
	require.NoError(t, err)
	assert.Equal(t, "notes.json", notes.Name)
	assert.Equal(t, "Notes", notes.Description)
	assert.Equal(t, int64(7), notes.Size)
	assert.Equal(t, "application/json", notes.Type)
	inSession, err := c.CreateResource(ctx, schema.ResourceCreateRequest{Session: session.String(), Type: "text/plain"}, "", strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Equal(t, session, *inSession.Session)
	assert.Equal(t, "text/plain", inSession.Type)

	// Not in a session which doesn't exist, or without content
	_, err = c.CreateResource(ctx, schema.ResourceCreateRequest{Session: uuid.New().String()}, "", strings.NewReader("hello"))
	assert.Error(t, err)
	_, err = c.CreateResource(ctx, schema.ResourceCreateRequest{}, "", nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Listed, on their own, or in the session
	list, err := c.ListResources(ctx, schema.ResourceListRequest{})
	require.NoError(t, err)
	require.Equal(t, uint(1), list.Count)
	assert.Equal(t, notes.Hash, list.Body[0].Hash)
	list, err = c.ListResources(ctx, schema.ResourceListRequest{Session: session.String()})
	require.NoError(t, err)
	require.Equal(t, uint(1), list.Count)
	assert.Equal(t, inSession.Hash, list.Body[0].Hash)

	// Got, and its content read, in its session only
	got, err := c.GetResource(ctx, inSession.Hash, schema.ResourceRequest{Session: session.String()})
	require.NoError(t, err)
	assert.Equal(t, inSession.Hash, got.Hash)
	_, err = c.GetResource(ctx, inSession.Hash, schema.ResourceRequest{})
	assert.Error(t, err, "it isn't on its own")
	var content bytes.Buffer
	contentType, err := c.ReadResource(ctx, notes.Hash, schema.ResourceRequest{}, &content)
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, content.String())
	assert.Equal(t, "application/json", contentType)

	// Content which can run scripts, such as HTML, or SVG, isn't sniffed,
	// and is sandboxed, so it can't use the UI's origin, when it's opened
	// in the browser
	for contentType, sandboxed := range map[string]bool{"text/html": true, "image/svg+xml": true, "image/png": false} {
		active, err := c.CreateResource(ctx, schema.ResourceCreateRequest{Type: contentType}, "", strings.NewReader("<script>"+contentType+"</script>"))
		require.NoError(t, err)
		response, err := http.Get(server.URL + test.Prefix + "/resource/" + active.Hash + "/content")
		require.NoError(t, err)
		response.Body.Close()
		assert.Equal(t, "nosniff", response.Header.Get("X-Content-Type-Options"), contentType)
		if sandboxed {
			assert.Equal(t, "sandbox", response.Header.Get("Content-Security-Policy"), contentType)
		} else {
			assert.Empty(t, response.Header.Get("Content-Security-Policy"), contentType)
		}
	}

	// Deleted
	deleted, err := c.DeleteResource(ctx, notes.Hash, schema.ResourceRequest{})
	require.NoError(t, err)
	assert.Equal(t, notes.Hash, deleted.Hash)
	_, err = c.GetResource(ctx, notes.Hash, schema.ResourceRequest{})
	assert.Error(t, err)
}

func Test_Upload(t *testing.T) {
	r, err := resource.New(resource.NewStore(), resource.WithMaxSize(1024))
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router)
	})
	post := func(t *testing.T, query, contentType string, body io.Reader) (*http.Response, schema.Resource) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+test.Prefix+"/resource"+query, body)
		require.NoError(t, err)
		if contentType != "" {
			req.Header.Set(types.ContentTypeHeader, contentType)
		}
		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer response.Body.Close()
		var created schema.Resource
		if response.StatusCode < 300 {
			require.NoError(t, json.NewDecoder(response.Body).Decode(&created))
		}
		return response, created
	}

	// The request body is the content, whose type is the request's, and
	// what it is is the query, which is created, and then used again
	response, created := post(t, "?name=data.json", "application/json", strings.NewReader(`{"a":1}`))
	assert.Equal(t, http.StatusCreated, response.StatusCode)
	assert.Equal(t, "data.json", created.Name)
	assert.Equal(t, "application/json", created.Type)
	response, again := post(t, "?description=Again", "application/json", strings.NewReader(`{"a":1}`))
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, created.Hash, again.Hash)
	assert.Equal(t, "Again", again.Description)

	// Content which is too large, a session which isn't a UUID, and a type
	// which isn't one, aren't
	response, _ = post(t, "", "text/plain", strings.NewReader(strings.Repeat("x", 2048)))
	assert.Equal(t, http.StatusRequestEntityTooLarge, response.StatusCode)
	response, _ = post(t, "?session=x", "text/plain", strings.NewReader("hello"))
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	response, _ = post(t, "?type=not+a+type", "", strings.NewReader("hello"))
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	// Content which can run scripts in a browser, such as HTML, is served
	// sandboxed, and isn't sniffed
	_, html := post(t, "?name=page.html", "text/html", strings.NewReader("<script>alert(1)</script>"))
	page, err := http.Get(server.URL + test.Prefix + "/resource/" + html.Hash + "/content")
	require.NoError(t, err)
	page.Body.Close()
	assert.Equal(t, "sandbox", page.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", page.Header.Get("X-Content-Type-Options"))

	// A hash which isn't one isn't found
	get, err := http.Get(server.URL + test.Prefix + "/resource/xyz")
	require.NoError(t, err)
	get.Body.Close()
	assert.Equal(t, http.StatusBadRequest, get.StatusCode)
}
