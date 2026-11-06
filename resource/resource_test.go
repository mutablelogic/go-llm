package resource

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// pngHeader is the start of a PNG image, whose type is detected
const pngHeader = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

// newManager returns a manager of resources in memory, whose clock is the time, which
// the test moves on, with the sessions which exist
func newManager(t *testing.T, sessions ...uuid.UUID) (*Manager, *time.Time, Store) {
	t.Helper()
	store := NewStore()
	exists := func(_ context.Context, id uuid.UUID) error {
		for _, session := range sessions {
			if session == id {
				return nil
			}
		}
		return llm.ErrNotFound.Withf("session %q", id)
	}
	r, err := New(store, WithSessions(exists), WithMaxSize(1024))
	require.NoError(t, err)
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	return r, &now, store
}

// read returns the content of the resource with the key
func read(t *testing.T, r *Manager, key schema.ResourceKey) string {
	t.Helper()
	_, content, err := r.Open(context.Background(), key)
	require.NoError(t, err)
	defer content.Close()
	data, err := io.ReadAll(content)
	require.NoError(t, err)
	return string(data)
}

func Test_Create(t *testing.T) {
	ctx := context.Background()
	session := uuid.New()
	r, now, store := newManager(t, session)

	// A resource is its content's hash, its size, and what it is, whose type
	// is detected from its name, or its content
	text, created, err := r.Create(ctx, schema.ResourceMeta{Name: "/tmp/notes.txt", Description: "Notes"}, strings.NewReader("hello"))
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", text.Hash)
	assert.Equal(t, "resource://"+text.Hash, text.URI())
	assert.Equal(t, int64(5), text.Size)
	assert.Equal(t, "notes.txt", text.Name, "the name is the file's, without its directories")
	assert.Equal(t, "text/plain; charset=utf-8", text.Type)
	assert.Equal(t, *now, text.Created)
	image, _, err := r.Create(ctx, schema.ResourceMeta{Type: "application/octet-stream"}, strings.NewReader(pngHeader))
	require.NoError(t, err)
	assert.Equal(t, "image/png", image.Type)
	given, _, err := r.Create(ctx, schema.ResourceMeta{Type: "Application/PDF"}, strings.NewReader("%PDF-1.7"))
	require.NoError(t, err)
	assert.Equal(t, "application/pdf", given.Type)

	// The same content, in its session, is the same resource, which is used
	// again, with what's given, and the same content in a session is another
	// resource, with the one content
	*now = now.Add(time.Minute)
	again, created, err := r.Create(ctx, schema.ResourceMeta{Name: "hello.txt"}, strings.NewReader("hello"))
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, "hello.txt", again.Name)
	assert.Equal(t, "Notes", again.Description, "what isn't given is kept")
	assert.Equal(t, *now, again.Used)
	inSession, created, err := r.Create(ctx, schema.ResourceMeta{Session: &session}, strings.NewReader("hello"))
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, text.Hash, inSession.Hash)
	contents, err := store.Contents(ctx)
	require.NoError(t, err)
	assert.Len(t, contents, 3, "the content is kept once")

	// A session which doesn't exist, content which is too large, and a type
	// which isn't one, aren't
	other := uuid.New()
	_, _, err = r.Create(ctx, schema.ResourceMeta{Session: &other}, strings.NewReader("hello"))
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, _, err = r.Create(ctx, schema.ResourceMeta{}, strings.NewReader(strings.Repeat("x", 1025)))
	var tooLarge *http.MaxBytesError
	assert.True(t, errors.As(err, &tooLarge))
	_, _, err = r.Create(ctx, schema.ResourceMeta{Type: "not a type;"}, strings.NewReader("hello"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = r.Create(ctx, schema.ResourceMeta{}, nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	contents, err = store.Contents(ctx)
	require.NoError(t, err)
	assert.Len(t, contents, 3, "content which is too large isn't kept")
}

func Test_List(t *testing.T) {
	ctx := context.Background()
	session := uuid.New()
	r, now, _ := newManager(t, session)
	for _, content := range []string{"one", "two", "three"} {
		_, _, err := r.Create(ctx, schema.ResourceMeta{}, strings.NewReader(content))
		require.NoError(t, err)
		*now = now.Add(time.Minute)
	}
	_, _, err := r.Create(ctx, schema.ResourceMeta{Session: &session}, strings.NewReader("four"))
	require.NoError(t, err)

	// Every resource, in a session, or on its own, the most recently used
	// first, a page at a time, or those of a session
	list, err := r.List(ctx, schema.ResourceListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(4), list.Count)
	require.Len(t, list.Body, 4)
	assert.Equal(t, int64(4), list.Body[0].Size, "four, in the session, is the most recent")
	assert.Equal(t, session, *list.Body[0].Session)
	assert.Equal(t, int64(5), list.Body[1].Size)
	assert.Nil(t, list.Body[1].Session, "three is on its own")
	list, err = r.List(ctx, schema.ResourceListRequest{Offset: 2, Limit: types.Ptr(uint(1))})
	require.NoError(t, err)
	assert.Equal(t, uint(4), list.Count)
	require.Len(t, list.Body, 1)
	assert.Equal(t, int64(3), list.Body[0].Size)
	list, err = r.List(ctx, schema.ResourceListRequest{Session: session.String()})
	require.NoError(t, err)
	assert.Equal(t, uint(1), list.Count)
	list, err = r.List(ctx, schema.ResourceListRequest{Offset: 10})
	require.NoError(t, err)
	assert.Empty(t, list.Body)
	_, err = r.List(ctx, schema.ResourceListRequest{Session: "x"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Read(t *testing.T) {
	ctx := context.Background()
	session := uuid.New()
	r, now, _ := newManager(t, session)
	created, _, err := r.Create(ctx, schema.ResourceMeta{Name: "a.txt", Session: &session}, strings.NewReader("hello"))
	require.NoError(t, err)

	// Its content is read, and it's used
	*now = now.Add(time.Hour)
	assert.Equal(t, "hello", read(t, r, created.Key()))
	got, err := r.Get(ctx, created.Key())
	require.NoError(t, err)
	assert.Equal(t, *now, got.Used)

	// As an llm.Resource, by its URI, in its session only
	resource, err := r.Resource(ctx, created.URI(), &session)
	require.NoError(t, err)
	assert.Equal(t, created.URI(), resource.URI())
	assert.Equal(t, "a.txt", resource.Name())
	assert.Equal(t, "text/plain; charset=utf-8", resource.Type())
	data, err := llm.ReadAll(ctx, resource)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	_, err = r.Resource(ctx, created.URI(), nil)
	assert.ErrorIs(t, err, llm.ErrNotFound, "it isn't on its own")
	_, err = r.Resource(ctx, "https://example.com", &session)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_Delete(t *testing.T) {
	ctx := context.Background()
	one, two := uuid.New(), uuid.New()
	r, _, store := newManager(t, one, two)
	b := broadcaster.New()
	r.broadcaster = b
	events := make(chan eventschema.Event, 10)
	_, err := b.Watch(ctx, events)
	require.NoError(t, err)

	first, _, err := r.Create(ctx, schema.ResourceMeta{Session: &one}, strings.NewReader("shared"))
	require.NoError(t, err)
	_, _, err = r.Create(ctx, schema.ResourceMeta{Session: &two}, strings.NewReader("shared"))
	require.NoError(t, err)
	_, _, err = r.Create(ctx, schema.ResourceMeta{Session: &one}, strings.NewReader("only"))
	require.NoError(t, err)

	// Deleting a resource, or a session's, only unlinks its content, and
	// publishes that it was deleted
	deleted, err := r.Delete(ctx, first.Key())
	require.NoError(t, err)
	assert.Equal(t, first.Hash, deleted.Hash)
	_, err = r.Delete(ctx, first.Key())
	assert.ErrorIs(t, err, llm.ErrNotFound)
	n, err := r.DeleteSession(ctx, one)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	contents, err := store.Contents(ctx)
	require.NoError(t, err)
	assert.Len(t, contents, 2, "content is kept, until it's cleaned")

	// Cleaning removes the content which no resource refers to, and keeps
	// what the other session's resource does
	r.Clean(ctx, nil)
	contents, err = store.Contents(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{first.Hash}, contents)
	n, err = r.DeleteSession(ctx, two)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	r.Clean(ctx, nil)
	contents, err = store.Contents(ctx)
	require.NoError(t, err)
	assert.Empty(t, contents)

	// Each change was published, with its session
	var actions []eventschema.EventAction
	for len(events) > 0 {
		event := <-events
		assert.Equal(t, schema.Name, event.Schema)
		actions = append(actions, event.Action)
	}
	assert.Equal(t, []eventschema.EventAction{eventschema.EventCreate, eventschema.EventCreate, eventschema.EventCreate, eventschema.EventDelete, eventschema.EventDelete, eventschema.EventDelete}, actions)
}

func Test_Clean(t *testing.T) {
	ctx := context.Background()
	session, gone := uuid.New(), uuid.New()
	r, now, store := newManager(t, session, gone)
	alone, _, err := r.Create(ctx, schema.ResourceMeta{}, strings.NewReader("alone"))
	require.NoError(t, err)
	kept, _, err := r.Create(ctx, schema.ResourceMeta{Session: &session}, strings.NewReader("kept"))
	require.NoError(t, err)
	_, _, err = r.Create(ctx, schema.ResourceMeta{Session: &gone}, strings.NewReader("gone"))
	require.NoError(t, err)
	_, _, err = store.Write(ctx, strings.NewReader("orphan"))
	require.NoError(t, err)

	// Nothing on its own is deleted before the retention, but content which
	// no resource refers to is, and a resource whose session doesn't exist
	r.session = func(_ context.Context, id uuid.UUID) error {
		if id == gone {
			return llm.ErrNotFound
		}
		return nil
	}
	*now = now.Add(schema.DefaultRetention - time.Minute)
	r.Clean(ctx, nil)
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	contents, err := store.Contents(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{alone.Hash, kept.Hash}, contents)

	// Using it keeps it, until it isn't used for the retention
	_, err = r.Resource(ctx, alone.URI(), nil)
	require.NoError(t, err)
	*now = now.Add(schema.DefaultRetention - time.Minute)
	r.Clean(ctx, nil)
	_, err = store.Get(ctx, alone.Key())
	require.NoError(t, err)
	*now = now.Add(time.Minute)
	r.Clean(ctx, nil)
	_, err = store.Get(ctx, alone.Key())
	assert.ErrorIs(t, err, llm.ErrNotFound)
	contents, err = store.Contents(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{kept.Hash}, contents, "a resource in a session is kept")
}

func Test_Run(t *testing.T) {
	// A session which is deleted has its resources deleted
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := uuid.New()
	r, _, store := newManager(t, session)
	b := broadcaster.New()
	r.broadcaster = b
	_, _, err := r.Create(ctx, schema.ResourceMeta{Session: &session}, strings.NewReader("hello"))
	require.NoError(t, err)

	done := make(chan error)
	go func() { done <- r.Run(ctx, nil) }()
	require.Eventually(t, func() bool {
		b.Publish(eventschema.Event{Action: eventschema.EventDelete, Schema: sessionSchema, Table: sessionTable, Row: session.String()})
		list, err := store.List(ctx)
		return err == nil && len(list) == 0
	}, 5*time.Second, 50*time.Millisecond)
	cancel()
	assert.NoError(t, <-done)
}

func Test_Schema(t *testing.T) {
	// A key's hash is a SHA-256, and its session a UUID, or none
	key, err := schema.ResourceRequest{}.Key(strings.Repeat("A", 64))
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("a", 64), key.Hash)
	assert.Nil(t, key.Session)
	_, err = schema.ResourceRequest{}.Key("abc")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = schema.ResourceRequest{Session: "x"}.Key(strings.Repeat("a", 64))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	hash, err := schema.ParseURI("resource://" + strings.Repeat("b", 64))
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("b", 64), hash)
}

func Test_Link(t *testing.T) {
	ctx := context.Background()
	session := uuid.New()
	r, _, store := newManager(t, session)
	alone, _, err := r.Create(ctx, schema.ResourceMeta{Name: "a.png", Description: "A picture"}, strings.NewReader(pngHeader))
	require.NoError(t, err)

	// A resource on its own is linked to a session, as the session's, with
	// the one content, and what it is, and linking it again is the same
	linked, err := r.Link(ctx, alone.URI(), session)
	require.NoError(t, err)
	assert.Equal(t, alone.Hash, linked.Hash)
	assert.Equal(t, session, *linked.Session)
	assert.Equal(t, "a.png", linked.Name)
	assert.Equal(t, "A picture", linked.Description)
	again, err := r.Link(ctx, alone.URI(), session)
	require.NoError(t, err)
	assert.Equal(t, linked.Key(), again.Key())
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	contents, err := store.Contents(ctx)
	require.NoError(t, err)
	assert.Len(t, contents, 1)

	// It's found in the session, and stays when the one on its own goes
	_, err = r.Delete(ctx, alone.Key())
	require.NoError(t, err)
	r.Clean(ctx, nil)
	_, err = r.Resource(ctx, alone.URI(), &session)
	require.NoError(t, err)

	// A resource which isn't on its own, or a session which doesn't exist,
	// isn't linked
	_, err = r.Link(ctx, "resource://"+strings.Repeat("0", 64), session)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	other, _, err := r.Create(ctx, schema.ResourceMeta{}, strings.NewReader("other"))
	require.NoError(t, err)
	_, err = r.Link(ctx, other.URI(), uuid.New())
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = r.Link(ctx, "https://example.com", session)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

// racingStore is a store which calls a function as a resource is deleted,
// before it's checked, as if it were changed at the same time
type racingStore struct {
	Store
	onDelete func()
}

func (s racingStore) Delete(ctx context.Context, key schema.ResourceKey, check func(schema.Resource) error) (schema.Resource, error) {
	if s.onDelete != nil {
		s.onDelete()
	}
	return s.Store.Delete(ctx, key, check)
}

// sessionReader reads the content, and deletes the session once it's read,
// as if it were deleted as a slow upload was read
type sessionReader struct {
	io.Reader
	deleted *bool
}

func (r sessionReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		*r.deleted = true
	}
	return n, err
}

func Test_Races(t *testing.T) {
	ctx := context.Background()

	t.Run("Clean", func(t *testing.T) {
		// A resource which is used as it's cleaned isn't deleted
		r, now, _ := newManager(t)
		alone, _, err := r.Create(ctx, schema.ResourceMeta{}, strings.NewReader("alone"))
		require.NoError(t, err)
		*now = now.Add(schema.DefaultRetention + time.Minute)
		r.store = racingStore{Store: r.store, onDelete: func() {
			_, err := r.Resource(ctx, alone.URI(), nil)
			require.NoError(t, err)
		}}
		r.Clean(ctx, nil)
		_, err = r.Get(ctx, alone.Key())
		assert.NoError(t, err, "it was used, so it's kept")
	})

	t.Run("Session", func(t *testing.T) {
		// A resource whose session is deleted as it's uploaded isn't created
		session := uuid.New()
		deleted := false
		r, _, store := newManager(t)
		r.session = func(_ context.Context, id uuid.UUID) error {
			if id == session && !deleted {
				return nil
			}
			return llm.ErrNotFound
		}
		_, _, err := r.Create(ctx, schema.ResourceMeta{Session: &session}, sessionReader{strings.NewReader("slow"), &deleted})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		list, err := store.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
	})
}

func Test_Types(t *testing.T) {
	ctx := context.Background()
	r, _, _ := newManager(t)

	// A resource's type is kept when it's created again without one, or with
	// application/octet-stream, and changed when one is given
	created, _, err := r.Create(ctx, schema.ResourceMeta{Name: "notes.json"}, strings.NewReader(`{"a":1}`))
	require.NoError(t, err)
	assert.Equal(t, "application/json", created.Type)
	again, _, err := r.Create(ctx, schema.ResourceMeta{Type: "application/octet-stream"}, strings.NewReader(`{"a":1}`))
	require.NoError(t, err)
	assert.Equal(t, "application/json", again.Type)
	given, _, err := r.Create(ctx, schema.ResourceMeta{Type: "text/plain"}, strings.NewReader(`{"a":1}`))
	require.NoError(t, err)
	assert.Equal(t, "text/plain", given.Type)

	// A type must be a type, with a subtype
	_, _, err = r.Create(ctx, schema.ResourceMeta{Type: "attachment"}, strings.NewReader("x"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_ReadUses(t *testing.T) {
	ctx := context.Background()
	r, now, _ := newManager(t)
	created, _, err := r.Create(ctx, schema.ResourceMeta{}, strings.NewReader("hello"))
	require.NoError(t, err)
	resource, err := r.Resource(ctx, created.URI(), nil)
	require.NoError(t, err)

	// Each read of it uses it, and it can't be read once it's deleted
	*now = now.Add(time.Hour)
	_, err = llm.ReadAll(ctx, resource)
	require.NoError(t, err)
	got, err := r.Get(ctx, created.Key())
	require.NoError(t, err)
	assert.Equal(t, *now, got.Used)
	_, err = r.Delete(ctx, created.Key())
	require.NoError(t, err)
	_, err = llm.ReadAll(ctx, resource)
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
