package registry

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
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
