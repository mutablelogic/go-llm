package registry

import (
	"context"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ReplyStatuses(t *testing.T) {
	// Two replies of a session at once are each thinking, or not, on their
	// own, and each logs "Thinking", once, to its own stream, so one doesn't
	// change the other's
	ctx := context.Background()
	r, err := New()
	require.NoError(t, err)
	id := uuid.New()
	var logs [2][]string
	stream := func(i int) llm.GeneratorStreamFn {
		return func(what llm.StreamType, text string) {
			if what == llm.StreamLog {
				logs[i] = append(logs[i], text)
			}
		}
	}
	replied1, replied2 := r.replying(id), r.replying(id)
	_, first, thought1 := r.thinkingStream(ctx, id, stream(0))
	ctx2, second, thought2 := r.thinkingStream(ctx, id, stream(1))
	assert.Equal(t, schema.SessionReplying, r.sessionStatus(id))

	first(llm.StreamThinking, "Hmm")
	second(llm.StreamThinking, "Hmm")
	assert.Equal(t, []string{ThinkingLog}, logs[0])
	assert.Equal(t, []string{ThinkingLog}, logs[1], "the second reply logs it too")

	// One writing its reply doesn't stop the other thinking
	first(llm.StreamText, "Sunny")
	assert.Equal(t, schema.SessionThinking, r.sessionStatus(id))

	// The other's tools running, and its thinking again, don't log again
	toolsRunning(ctx2)
	second(llm.StreamThinking, "Again")
	assert.Len(t, logs[1], 1)

	// One which replies without writing text isn't thinking, once it has
	thought2()
	assert.Equal(t, schema.SessionReplying, r.sessionStatus(id))
	second(llm.StreamThinking, "Late")
	assert.Equal(t, schema.SessionReplying, r.sessionStatus(id), "a reply which has replied isn't thinking again")
	thought1()
	replied1()
	replied2()
	assert.Equal(t, schema.SessionIdle, r.sessionStatus(id))

	// A reply which isn't streamed has no status of its own, and tools which
	// run without one do nothing
	_, none, thought := r.thinkingStream(ctx, id, nil)
	assert.Nil(t, none)
	thought()
	toolsRunning(ctx)
	assert.Equal(t, schema.SessionIdle, r.sessionStatus(id))
}
