package registry_test

import (
	"context"
	"regexp"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Times(t *testing.T) {
	ctx := context.Background()
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	sent := regexp.MustCompile(`^\(\w+day \d{4}-\d{2}-\d{2}T\d{2}:\d{2}\+0[12]:00\)$`)

	// A chat, with the tools of a namespace, or not
	chat := func(t *testing.T, tools ...string) (*scripted, *schema.Session, *registry.Registry) {
		t.Helper()
		sessions := test.NewSessionStore()
		tk := test.NewToolkit(t)
		require.NoError(t, tk.Add(toolkit.NewNamespace("assistant", "", readTool{})))
		require.NoError(t, tk.Add(toolkit.NewNamespace("files", "", readTool{})))
		p := newScripted(text("Hello."), text("Still here."))
		r, err := registry.New(registry.WithSessionStore(sessions), registry.WithToolkit(tk), registry.WithTimes(func() *time.Location { return berlin }, "assistant"))
		require.NoError(t, err)
		require.NoError(t, r.Add("", p))
		session, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model"), Tools: tools}})
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Hi")}, nil)
		require.NoError(t, err)
		_, err = r.Chat(ctx, session.ID, []llm.Message{user(t, "Still there?")}, nil)
		require.NoError(t, err)
		return p, session, r
	}

	t.Run("Memory", func(t *testing.T) {
		// Each of the person's messages starts with when it was sent, in their
		// time zone, as it's sent, and the replies don't
		p, session, r := chat(t, "assistant:*")
		messages := p.asked[1].messages
		require.Len(t, messages, 3)
		for i, want := range []string{"Hi", "Still there?"} {
			message := messages[i*2].(*providerschema.Message)
			require.Len(t, message.Content, 2)
			assert.Regexp(t, sent, types.Value(message.Content[0].Text))
			assert.Equal(t, want, types.Value(message.Content[1].Text))
		}
		assert.Equal(t, "Hello.", messages[1].Text())

		// Each message keeps when it was added, without the note
		stored, err := r.ListSessionMessages(ctx, session.ID, schema.MessageListRequest{})
		require.NoError(t, err)
		require.Len(t, stored.Body, 4)
		for _, message := range stored.Body {
			assert.WithinDuration(t, time.Now(), message.Time, time.Minute, message.Role())
			assert.NotRegexp(t, sent, message.Text())
		}
		assert.Equal(t, "Hi", stored.Body[0].Text())
	})

	t.Run("Without", func(t *testing.T) {
		// Without the namespace's tools, they aren't told when, though it's
		// kept
		p, _, _ := chat(t, "files:*")
		for _, message := range p.asked[1].messages {
			assert.NotRegexp(t, sent, message.Text())
		}
		assert.Equal(t, "Hi", p.asked[1].messages[0].Text())
	})
}
