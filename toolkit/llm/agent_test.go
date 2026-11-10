package llm_test

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
	llmtools "github.com/mutablelogic/go-llm/toolkit/llm"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// agents is a registry's agents, two at a time, a page
type agents struct {
	byName map[string]registryschema.Agent
}

func (a *agents) names() []string {
	var names []string
	for name := range a.byName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (a *agents) ListAgents(_ context.Context, req registryschema.AgentListRequest) (*registryschema.AgentList, error) {
	names := a.names()
	list := &registryschema.AgentList{AgentListRequest: req, Count: uint(len(names))}
	for i := req.Offset; i < uint(len(names)) && i < req.Offset+2; i++ {
		list.Body = append(list.Body, a.byName[names[i]])
	}
	return list, nil
}

// GetAgent resolves a name as the registry does: a bare one, or one with
// the agent's namespace
func (a *agents) GetAgent(_ context.Context, ref string) (*registryschema.Agent, error) {
	name := ref
	namespace, bare, qualified := strings.Cut(ref, ":")
	if qualified {
		name = bare
	}
	if agent, ok := a.byName[name]; ok && (!qualified || agent.Namespace() == namespace) {
		return &agent, nil
	}
	return nil, llm.ErrNotFound.Withf("agent %q", name)
}

func (a *agents) CreateAgent(_ context.Context, meta registryschema.AgentMeta) (*registryschema.Agent, bool, error) {
	agent := registryschema.Agent{AgentMeta: meta}
	a.byName[meta.Name] = agent
	return &agent, true, nil
}

func (a *agents) UpdateAgent(_ context.Context, name string, meta registryschema.AgentMeta) (*registryschema.Agent, error) {
	agent := a.byName[name]
	agent.AgentMeta = agent.AgentMeta.Merge(meta)
	a.byName[name] = agent
	return &agent, nil
}

func Test_Agent(t *testing.T) {
	ctx := context.Background()
	store := &agents{byName: map[string]registryschema.Agent{}}
	add := func(meta registryschema.AgentMeta) { store.byName[meta.Name] = registryschema.Agent{AgentMeta: meta} }
	add(registryschema.AgentMeta{PromptMeta: registryschema.PromptMeta{Name: "summarize", Title: types.Ptr("Summarize")}, GeneratorMeta: registryschema.GeneratorMeta{Model: types.Ptr("eliza")}})
	add(registryschema.AgentMeta{PromptMeta: registryschema.PromptMeta{Name: "translate", Builtin: types.Ptr(true)}})
	add(registryschema.AgentMeta{PromptMeta: registryschema.PromptMeta{Name: "compact", Builtin: types.Ptr(true)}})
	add(registryschema.AgentMeta{PromptMeta: registryschema.PromptMeta{Name: "event", Builtin: types.Ptr(true)}})
	add(registryschema.AgentMeta{PromptMeta: registryschema.PromptMeta{Name: "review_pr", Origin: "github"}})
	add(registryschema.AgentMeta{PromptMeta: registryschema.PromptMeta{Name: "weather"}})

	ns, err := llmtools.New(func() llmtools.Models { return &models{} }, llmtools.WithAgents(func() llmtools.Agents { return store }))
	require.NoError(t, err)
	var tool llm.Tool
	for _, each := range ns.Tools() {
		if each.Name() == "agent" {
			tool = each
		}
	}
	require.NotNil(t, tool, "with agents, there's an agent tool")
	run := func(input string) (any, error) {
		t.Helper()
		return tool.Run(ctx, json.RawMessage(input))
	}

	// Without agents, there's no tool; with none yet, it's unavailable
	without, err := llmtools.New(func() llmtools.Models { return &models{} })
	require.NoError(t, err)
	for _, each := range without.Tools() {
		assert.NotEqual(t, "agent", each.Name())
	}
	_, err = llmtools.New(func() llmtools.Models { return &models{} }, llmtools.WithAgents(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	t.Run("List", func(t *testing.T) {
		// Every page, of the agent namespace, and the built-in agents chats
		// call, but not the server's own, or connectors' prompts
		result, err := run(`{"action":"list"}`)
		require.NoError(t, err)
		data, err := json.Marshal(result)
		require.NoError(t, err)
		var list []map[string]any
		require.NoError(t, json.Unmarshal(data, &list))
		var tools []string
		for _, agent := range list {
			tools = append(tools, agent["tool"].(string))
		}
		assert.Equal(t, []string{"agent:summarize", "builtin:translate", "agent:weather"}, tools)
		assert.Equal(t, "agent", list[0]["namespace"])
		assert.Equal(t, "Summarize", list[0]["title"])
		assert.Equal(t, "eliza", list[0]["model"])
		assert.Equal(t, "builtin", list[1]["namespace"])
	})

	t.Run("Get", func(t *testing.T) {
		result, err := run(`{"action":"get","name":"agent:summarize"}`)
		require.NoError(t, err)
		meta, ok := result.(registryschema.AgentMeta)
		require.True(t, ok)
		assert.Equal(t, "summarize", meta.Name)

		result, err = run(`{"action":"get","name":"translate"}`)
		require.NoError(t, err, "a built-in agent")
		assert.Equal(t, "translate", result.(registryschema.AgentMeta).Name)
		_, err = run(`{"action":"get","name":"builtin:translate"}`)
		require.NoError(t, err)
		_, err = run(`{"action":"get","name":"compact"}`)
		assert.ErrorIs(t, err, llm.ErrForbidden, "only the server runs it")
		_, err = run(`{"action":"get","name":"review_pr"}`)
		assert.ErrorIs(t, err, llm.ErrForbidden, "a connector's prompt")
		_, err = run(`{"action":"get","name":"missing"}`)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = run(`{"action":"get"}`)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Create", func(t *testing.T) {
		_, err := run(`{"action":"create","name":"haiku","description":"Writes a haiku","prompt":"A haiku about {{ .topic }}","input":{"type":"object","properties":{"topic":{"type":"string"}}},"tools":["weatherapi:*"]}`)
		require.NoError(t, err)
		haiku := store.byName["haiku"]
		assert.Equal(t, "Writes a haiku", types.Value(haiku.Description))
		assert.Equal(t, "A haiku about {{ .topic }}", types.Value(haiku.Prompt))
		assert.Equal(t, []string{"weatherapi:*"}, haiku.Tools)
		assert.JSONEq(t, `{"type":"object","properties":{"topic":{"type":"string"}}}`, string(haiku.Input))

		_, err = run(`{"action":"create","name":"summarize","prompt":"x"}`)
		assert.ErrorIs(t, err, llm.ErrConflict, "it exists, so it's updated")
		_, err = run(`{"action":"create","name":"builtin:x","prompt":"x"}`)
		assert.ErrorIs(t, err, llm.ErrForbidden, "only in the agent namespace")
		_, err = run(`{"action":"create","prompt":"x"}`)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Update", func(t *testing.T) {
		_, err := run(`{"action":"update","name":"summarize","system":"Be brief"}`)
		require.NoError(t, err)
		summarize := store.byName["summarize"]
		assert.Equal(t, "Be brief", types.Value(summarize.System))
		assert.Equal(t, "Summarize", types.Value(summarize.Title), "fields not given are kept")
		assert.Equal(t, "eliza", types.Value(summarize.Model))

		_, err = run(`{"action":"update","name":"translate","model":"eliza"}`)
		assert.ErrorIs(t, err, llm.ErrForbidden)
		_, err = run(`{"action":"update","name":"missing","system":"x"}`)
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Action", func(t *testing.T) {
		_, err := run(`{"action":"delete","name":"summarize"}`)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = run(`{"action":`)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_AgentUnavailable(t *testing.T) {
	ns, err := llmtools.New(func() llmtools.Models { return &models{} }, llmtools.WithAgents(func() llmtools.Agents { return nil }))
	require.NoError(t, err)
	for _, tool := range ns.Tools() {
		if tool.Name() == "agent" {
			_, err := tool.Run(context.Background(), json.RawMessage(`{"action":"list"}`))
			assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		}
	}
}
