package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agents "github.com/mutablelogic/go-llm/agents"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// summarized returns the compact agent's reply, which is the summary, and
// title
func summarized(summary, title string) *providerschema.Message {
	return call(toolkit.RespondName, `{"result":{"summary":"`+summary+`","title":"`+title+`"}}`)
}

// newCompacting returns a registry with sessions, and the agents the server
// has, whose model replies with the replies
func newCompacting(t *testing.T, replies ...*providerschema.Message) (*registry.Registry, *scripted) {
	t.Helper()
	return newCompactingWith(t, nil, replies...)
}

// newCompactingWith returns a registry, as newCompacting does, with the
// options
func newCompactingWith(t *testing.T, opts []registry.Opt, replies ...*providerschema.Message) (*registry.Registry, *scripted) {
	t.Helper()
	ctx := context.Background()
	opts = append([]registry.Opt{registry.WithToolkit(test.NewToolkit(t)), registry.WithAgentStore(test.NewAgentStore()), registry.WithSessionStore(test.NewSessionStore())}, opts...)
	r, err := registry.New(opts...)
	require.NoError(t, err)
	p := newScripted(replies...)
	require.NoError(t, r.Add("", p))
	all, err := agents.All()
	require.NoError(t, err)
	for _, meta := range all {
		_, _, err := r.CreateAgent(ctx, meta)
		require.NoError(t, err)
	}
	return r, p
}

// marker returns the session's last marker, or nil
func marker(t *testing.T, r *registry.Registry, id uuid.UUID) *providerschema.Marker {
	t.Helper()
	list, err := r.ListSessionMessages(context.Background(), id, schema.MessageListRequest{})
	require.NoError(t, err)
	for i := len(list.Body) - 1; i >= 0; i-- {
		if list.Body[i].Marker != nil {
			return list.Body[i].Marker
		}
	}
	return nil
}

func Test_CompactSession(t *testing.T) {
	ctx := context.Background()
	r, p := newCompacting(t, text("It's 4°C in Berlin"), summarized("David asked about Berlin's weather, which is 4°C.", "Berlin weather"), text("You're welcome"), summarized("David thanked the assistant.", "Thanks"))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), System: types.Ptr("Be brief")}})
	require.NoError(t, err)

	// Nothing to compact, until there are messages
	_, err = r.CompactSession(ctx, session.ID)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "What's the weather in Berlin?")}, nil)
	require.NoError(t, err)

	// A marker is added, whose summary is written, in the background, with a
	// title, which is the session's, as it has none
	_, err = r.CompactSession(ctx, session.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Status == providerschema.MarkerSummarized
	}, 5*time.Second, 10*time.Millisecond)
	m := marker(t, r, session.ID)
	assert.Equal(t, "asked", m.Reason)
	assert.Equal(t, "David asked about Berlin's weather, which is 4°C.", m.Summary)
	assert.Equal(t, "Berlin weather", m.Title)
	got, err := r.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, "Berlin weather", types.Value(got.Title))

	// The compact agent was given the transcript, without tools
	prompt := p.asked[1].messages[0].Text()
	assert.Contains(t, prompt, "[user] What's the weather in Berlin?")
	assert.Contains(t, prompt, "[assistant] It's 4°C in Berlin")

	// The next reply is to the messages after the marker, with the summary
	// in the system prompt, after its own
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Thanks")}, nil)
	require.NoError(t, err)
	asked := p.asked[2]
	require.Len(t, asked.messages, 1)
	assert.Equal(t, "Thanks", asked.messages[0].Text())
	system, _ := llm.GetOpt[string](asked.opts, llm.SystemPromptKey)
	assert.True(t, strings.HasPrefix(system, "Be brief\n\n# Earlier in this conversation"), system)
	assert.Contains(t, system, "David asked about Berlin's weather, which is 4°C.")

	// The messages are all kept, with the marker between them
	list, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	var roles []string
	for _, message := range list.Body {
		roles = append(roles, message.Role())
	}
	assert.Equal(t, []string{"user", "assistant", "context", "user", "assistant"}, roles)

	// Compacting again, while its summary is written, is a conflict, and,
	// once it's written, with no messages since, is nothing to compact
	_, err = r.CompactSession(ctx, session.ID)
	require.NoError(t, err, "there are messages since")
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Status == providerschema.MarkerSummarized && m.Summary == "David thanked the assistant."
	}, 5*time.Second, 10*time.Millisecond)
	_, err = r.CompactSession(ctx, session.ID)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = r.CompactSession(ctx, uuid.New())
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_CompactTitle(t *testing.T) {
	ctx := context.Background()

	// The conversation's title is the summary's, each time it's compacted,
	// whatever it was
	r, _ := newCompacting(t, text("Hello"), summarized("A greeting.", "Greetings"), text("Again"), summarized("Two greetings.", "More greetings"))
	session, err := r.CreateSession(ctx, schema.SessionMeta{Title: types.Ptr("My chat"), GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	compactNow := func(summary string) {
		t.Helper()
		_, err := r.CompactSession(ctx, session.ID)
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			m := marker(t, r, session.ID)
			return m != nil && m.Summary == summary
		}, 5*time.Second, 10*time.Millisecond)
	}
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)
	compactNow("A greeting.")
	got, err := r.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, "Greetings", types.Value(got.Title), "the title it was given is replaced")

	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi again")}, nil)
	require.NoError(t, err)
	compactNow("Two greetings.")
	got, err = r.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, "More greetings", types.Value(got.Title), "and again")
}

func Test_CompactFailed(t *testing.T) {
	ctx := context.Background()

	// A summary which can't be written marks the marker failed, saying why,
	// and the conversation still starts after it
	r, p := newCompacting(t, text("Hello"))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)
	p.mu.Lock()
	p.err = errors.New("the model is unavailable")
	p.mu.Unlock()
	_, err = r.CompactSession(ctx, session.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Status == providerschema.MarkerFailed
	}, 5*time.Second, 10*time.Millisecond)
	assert.Contains(t, marker(t, r, session.ID).Error, "the model is unavailable")
}

func Test_CompactWhenFull(t *testing.T) {
	ctx := context.Background()

	// A conversation which fills most of its model's context, as of a reply,
	// is compacted, in the background
	r, p := newCompacting(t, text(strings.Repeat("word ", 40)), summarized("A long reply.", "Long"))
	p.input = types.Ptr(uint(16))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Say a lot")}, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Status != providerschema.MarkerPending
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, "full", marker(t, r, session.ID).Reason)
}

func Test_CompactWhileReplying(t *testing.T) {
	ctx := context.Background()

	// A summary which is written as a reply is, is kept, and the reply is
	// stored, after the marker
	r, p := newCompacting(t, text("First"), text("Second"), summarized("The first message.", "First"))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "One")}, nil)
	require.NoError(t, err)

	// The reply waits, and the session is compacted, and summarised, as it
	// does
	p.mu.Lock()
	p.delay = 300 * time.Millisecond
	p.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, err := r.Chat(ctx, session.ID, []llm.Message{user(t, "Two")}, func(llm.StreamType, string) {})
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	_, err = r.CompactSession(ctx, session.ID)
	require.NoError(t, err)
	require.NoError(t, <-done)
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Status != providerschema.MarkerPending
	}, 5*time.Second, 10*time.Millisecond)
	list, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	var roles []string
	for _, message := range list.Body {
		roles = append(roles, message.Role()+":"+message.Text())
	}
	assert.Equal(t, []string{"user:One", "assistant:First", "context:", "user:Two", "assistant:Second"}, roles)
}

func Test_NotFull(t *testing.T) {
	ctx := context.Background()

	// One which doesn't isn't
	r, p := newCompacting(t, text("Short"))
	p.input = types.Ptr(uint(1000))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	assert.Nil(t, marker(t, r, session.ID))
}

func Test_CompactWithTools(t *testing.T) {
	ctx := context.Background()

	// The compact agent is given the session's tools of the namespaces, such
	// as memory, which the weather tools stand in for, and no others
	for _, c := range []struct {
		tools []string
		want  []string
	}{
		{[]string{"*"}, []string{"weather__current", "weather__forecast"}},
		{[]string{"weather:current"}, []string{"weather__current"}},
		{nil, []string{}},
	} {
		r, p := newCompactingWith(t, []registry.Opt{registry.WithCompactTools("weather")}, text("Hello"), summarized("A greeting.", "Hello"))
		session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Tools: c.tools}})
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
		require.NoError(t, err)
		_, err = r.CompactSession(ctx, session.ID)
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			m := marker(t, r, session.ID)
			return m != nil && m.Status != providerschema.MarkerPending
		}, 5*time.Second, 10*time.Millisecond)
		p.mu.Lock()
		got := slices.DeleteFunc(slices.Clone(p.asked[1].tools), func(name string) bool { return name == toolkit.RespondName })
		p.mu.Unlock()
		assert.Equal(t, c.want, got, c.tools)
	}
}

// withPendingMarker returns the session, with a marker whose summary is
// pending, as when the server stopped as it was written, after its messages
func withPendingMarker(t *testing.T, store *test.SessionStore, id uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	session, err := store.Get(ctx, id)
	require.NoError(t, err)
	marker := providerschema.Marker{ID: uuid.New(), Reason: "asked", Status: providerschema.MarkerPending}
	session.Messages = append(session.Messages, providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: providerschema.RoleContext, Content: []providerschema.ContentBlock{}}, Time: time.Now().UTC(), Marker: &marker})
	_, err = store.Put(ctx, session)
	require.NoError(t, err)
	return marker.ID
}

func Test_CompactPending(t *testing.T) {
	ctx := context.Background()
	store := test.NewSessionStore()
	r, err := registry.New(registry.WithToolkit(test.NewToolkit(t)), registry.WithAgentStore(test.NewAgentStore()), registry.WithSessionStore(store))
	require.NoError(t, err)
	require.NoError(t, r.Add("", newScripted(text("Hello"), summarized("A greeting.", "Greetings"))))
	all, err := agents.All()
	require.NoError(t, err)
	for _, meta := range all {
		_, _, err := r.CreateAgent(ctx, meta)
		require.NoError(t, err)
	}
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)
	pending := withPendingMarker(t, store, session.ID)

	// While its summary is being written, it isn't compacted again
	done := r.Summarizing(pending)
	_, err = r.CompactSession(ctx, session.ID)
	assert.ErrorIs(t, err, llm.ErrConflict)
	done()

	// One which isn't being written, such as when the server stopped as it
	// was, is written, rather than another marker added
	_, err = r.CompactSession(ctx, session.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Status == providerschema.MarkerSummarized
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, pending, marker(t, r, session.ID).ID)
	list, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
	require.NoError(t, err)
	assert.Len(t, list.Body, 3, "no other marker")
}

func Test_CompactFailedKeepsContext(t *testing.T) {
	ctx := context.Background()

	// A marker whose summary couldn't be written doesn't start the context,
	// so the messages before it are still sent, and the next summary has
	// them
	r, p := newCompacting(t, text("Hello"))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)
	p.mu.Lock()
	p.err = errors.New("the model is unavailable")
	p.mu.Unlock()
	_, err = r.CompactSession(ctx, session.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Status == providerschema.MarkerFailed
	}, 5*time.Second, 10*time.Millisecond)
	p.mu.Lock()
	p.err = nil
	p.replies = append(p.replies, text("Still here"), summarized("Hi, and still here.", "Hi"))
	p.mu.Unlock()
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Are you?")}, nil)
	require.NoError(t, err)
	asked := p.asked[len(p.asked)-1]
	require.Len(t, asked.messages, 3, "the messages before the failed marker, without it")
	assert.Equal(t, "Hi", asked.messages[0].Text())

	_, err = r.CompactSession(ctx, session.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Status == providerschema.MarkerSummarized
	}, 5*time.Second, 10*time.Millisecond)
	prompt := p.asked[len(p.asked)-1].messages[0].Text()
	assert.Contains(t, prompt, "[user] Hi", "the summary has what came before the failed marker")
	assert.Contains(t, prompt, "[user] Are you?")
}

func Test_CompactContextWindow(t *testing.T) {
	ctx := context.Background()

	// The context_window option, which is smaller than the model's limit, or
	// is the only one, is the window a conversation fills
	r, _ := newCompacting(t, text("Hello"), summarized("A greeting.", "Hello"))
	session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Opt: map[string]json.RawMessage{"context_window": json.RawMessage("16")}}})
	require.NoError(t, err)
	_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		m := marker(t, r, session.ID)
		return m != nil && m.Reason == "full"
	}, 5*time.Second, 10*time.Millisecond)
}
