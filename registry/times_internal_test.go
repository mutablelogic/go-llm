package registry

import (
	"context"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_TimeTokens(t *testing.T) {
	// The conversation is measured with when the person's messages were
	// sent, which they start with, when the model is told, as it's sent,
	// though the messages' own tokens don't have them
	ctx := context.Background()
	measure := func(_ context.Context, text string) (uint, error) { return uint(len(text)), nil }
	at := time.Date(2026, 11, 7, 11, 38, 0, 0, time.UTC)
	say := func(role, value string, sent time.Time) providerschema.Message {
		return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: role, Content: []providerschema.ContentBlock{{Text: types.Ptr(value)}}}, Time: sent}
	}
	earlier := []llm.Message{
		&providerschema.Message{MessageMeta: say("user", "Hi", at).MessageMeta, Time: at, Tokens: 10},
		&providerschema.Message{MessageMeta: say("assistant", "Hello", at).MessageMeta, Time: at, Tokens: 10},
		&providerschema.Message{MessageMeta: say("user", "Before times", time.Time{}).MessageMeta, Tokens: 10},
	}
	fresh := []providerschema.Message{say("user", "Now", at)}

	without, err := newHistory(ctx, earlier, fresh, 0, measure, nil, nil)
	require.NoError(t, err)
	with, err := newHistory(ctx, earlier, fresh, 0, measure, nil, time.UTC)
	require.NoError(t, err)
	assert.Equal(t, without.size()+2*timeTokens, with.size(), "the person's two messages with times")
	assert.Equal(t, without.added[0].Tokens, with.added[0].Tokens, "the message's own tokens don't have it")

	// Which is at most the estimate
	note := withTimes([]llm.Message{&fresh[0]}, time.FixedZone("", -9*60*60-30*60))[0].(*providerschema.Message)
	tokens, err := measure(ctx, types.Value(note.Content[0].Text))
	require.NoError(t, err)
	assert.LessOrEqual(t, tokens/4, uint(timeTokens))
}
