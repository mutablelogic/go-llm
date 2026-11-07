package registry

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// agentStore is a store of any agent, with a model, for tests
type agentStore struct{ AgentStore }

func (agentStore) Get(_ context.Context, name string) (schema.Agent, error) {
	return schema.Agent{AgentMeta: schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: name, Prompt: types.Ptr("Run")}, GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}}}, nil
}

func Test_AgentChain(t *testing.T) {
	// The agents which are running are in the context, in the order they're
	// run, and each context has its own, with the most which can run in one
	// another, which a context can lower, but not raise
	ctx := context.Background()
	assert.Empty(t, agentChainOf(ctx).running)
	assert.Equal(t, uint(DefaultMaxDepth), agentChainOf(ctx).depth())
	first := withRunningAgent(ctx, "segment")
	second := withRunningAgent(first, "segment")
	other := withRunningAgent(first, "translate")
	assert.Equal(t, []string{"segment", "segment"}, agentChainOf(second).running)
	assert.Equal(t, []string{"segment", "translate"}, agentChainOf(other).running)
	assert.Equal(t, uint(2), agentChainOf(withMaxDepth(withMaxDepth(ctx, 2), 5)).depth())
	assert.Equal(t, uint(1), agentChainOf(withMaxDepth(withMaxDepth(ctx, 2), 1)).depth())
}

func Test_MaxDepth(t *testing.T) {
	r, err := New(WithAgentStore(agentStore{}))
	require.NoError(t, err)
	ctx := context.Background()

	// An agent can run in itself, as deep as the most agents which can run in
	// one another, which is three, unless max_depth says otherwise
	two := withRunningAgent(withRunningAgent(ctx, "segment"), "segment")
	_, err = r.RunAgent(two, "segment", schema.RunAgentRequest{}, nil)
	assert.ErrorIs(t, err, llm.ErrNotFound, "the third is run, without a provider of its model")
	_, err = r.RunAgent(withRunningAgent(two, "segment"), "segment", schema.RunAgentRequest{}, nil)
	assert.ErrorIs(t, err, llm.ErrForbidden, "a fourth isn't")
	assert.ErrorContains(t, err, "segment → segment → segment")

	// A session, or agent, whose max_depth is one runs no agent in another
	one := withMaxDepth(withRunningAgent(ctx, "segment"), 1)
	_, err = r.RunAgent(one, "summarize", schema.RunAgentRequest{}, nil)
	assert.ErrorIs(t, err, llm.ErrForbidden)
}

func Test_MaxDepthOption(t *testing.T) {
	// The agent loop reads max_depth, which isn't passed to the model, and
	// must be one or more
	loop, err := newAgentLoop(nil, nil, llm.WithMaxDepth(2), llm.WithOpt("temperature", 0.5))
	require.NoError(t, err)
	assert.Equal(t, uint(2), loop.maxDepth)
	o, err := llm.ApplyOpts(loop.opts...)
	require.NoError(t, err)
	assert.False(t, o.Has(llm.MaxDepthKey))
	assert.True(t, o.Has("temperature"))
	_, err = newAgentLoop(nil, nil, llm.WithOpt(llm.MaxDepthKey, 0))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = llm.ApplyOpts(llm.WithMaxDepth(0))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_DepthContext(t *testing.T) {
	// A loop without max_depth records the default, so an agent it runs,
	// whose max_depth is more, can't raise it, and one which has it lowers
	// it, but a loop without it, which an agent runs, keeps what its caller
	// says
	ctx := context.Background()
	session, err := newAgentLoop(nil, nil)
	require.NoError(t, err)
	child, err := newAgentLoop(nil, nil, llm.WithMaxDepth(10))
	require.NoError(t, err)
	lower, err := newAgentLoop(nil, nil, llm.WithMaxDepth(2))
	require.NoError(t, err)

	fromSession := session.depthContext(ctx)
	assert.Equal(t, uint(DefaultMaxDepth), agentChainOf(child.depthContext(fromSession)).depth())
	assert.Equal(t, uint(2), agentChainOf(lower.depthContext(fromSession)).depth())
	assert.Equal(t, uint(2), agentChainOf(session.depthContext(lower.depthContext(ctx))).depth())
	assert.Equal(t, uint(10), agentChainOf(child.depthContext(ctx)).depth(), "an agent run on its own")
}

func Test_AgentFinished(t *testing.T) {
	// A reply which didn't finish is an error, which says how it stopped
	reply := func(result providerschema.ResultType) []providerschema.Message {
		return []providerschema.Message{{Result: result}}
	}
	assert.NoError(t, agentFinished("a", reply(providerschema.ResultStop)))
	assert.NoError(t, agentFinished("a", reply("")))
	assert.ErrorIs(t, agentFinished("a", reply(providerschema.ResultMaxTokens)), llm.ErrMaxTokens)
	assert.ErrorIs(t, agentFinished("a", reply(providerschema.ResultBlocked)), llm.ErrRefusal)
	err := agentFinished("a", reply(providerschema.ResultMaxContext))
	assert.ErrorIs(t, err, llm.ErrBadParameter, "its input is too long")
	assert.ErrorContains(t, err, "too long for its model's context")
	for _, result := range []providerschema.ResultType{providerschema.ResultError, providerschema.ResultMaxIterations} {
		err := agentFinished("a", reply(result))
		assert.ErrorIs(t, err, llm.ErrInternalServerError, result)
		assert.ErrorContains(t, err, string(result))
	}
	assert.Error(t, agentFinished("a", nil))
}

// progressSession is a session which keeps the progress it's told
type progressSession struct {
	toolkit.Session
	progress []llm.Progress
}

func (s *progressSession) Progress(progress, total float64, message string) {
	s.progress = append(s.progress, llm.Progress{Progress: progress, Total: total, Message: message})
}

func Test_AgentStream(t *testing.T) {
	// An agent's progress, and its tools', is one count, which increases,
	// and its thinking, or reply, is said again after its tools' progress
	session := &progressSession{Session: toolkit.SessionFromContext(context.Background())}
	stream := agentStream("outer", session)
	stream(llm.StreamThinking, "Hmm")
	stream(llm.StreamThinking, " more")
	stream(llm.StreamText, "Let me")
	stream(llm.StreamProgress, llm.Progress{Progress: 1, Total: 4, Message: "Agent inner is thinking"}.Text())
	stream(llm.StreamText, "Done")
	stream(llm.StreamLog, "A line")
	stream(llm.StreamText, "Done again")
	var messages []string
	var last float64
	for _, p := range session.progress {
		assert.Greater(t, p.Progress, last, p.Message)
		assert.Zero(t, p.Total)
		last = p.Progress
		messages = append(messages, p.Message)
	}
	assert.Equal(t, []string{
		"Agent outer is thinking",
		"Agent outer is replying",
		"Agent inner is thinking",
		"Agent outer is replying",
		"Agent outer is replying",
	}, messages)
}

func Test_MeasureWithoutUsage(t *testing.T) {
	// The usage of a tool's result isn't sent to the model, so it isn't
	// measured as the conversation's
	h := &history{measure: func(_ context.Context, text string) (uint, error) { return uint(len(text)), nil }}
	result := func(usage *llm.Usage) *providerschema.Message {
		return &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{
			{ToolResult: &providerschema.ToolResult{ID: "1", Name: "agent__segment", Content: json.RawMessage(`"Done"`), Usage: usage}},
		}}}
	}
	with := result(&llm.Usage{InputTokens: 1000, OutputTokens: 500, Duration: 1e9})
	without, err := h.measureMessage(context.Background(), result(nil))
	require.NoError(t, err)
	measured, err := h.measureMessage(context.Background(), with)
	require.NoError(t, err)
	assert.Equal(t, without, measured)
	assert.NotNil(t, with.Content[0].ToolResult.Usage, "the message keeps its usage")
}
