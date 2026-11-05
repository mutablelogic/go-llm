package registry_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Agents(t *testing.T) {
	ctx := context.Background()
	newRegistry := func(t *testing.T) (*registry.Registry, *test.AgentStore) {
		t.Helper()
		store := test.NewAgentStore()
		r, err := registry.New(registry.WithAgentStore(store), registry.WithToolkit(test.NewToolkit(t)))
		require.NoError(t, err)
		p := newScripted()
		p.models = []string{"model", "other"}
		require.NoError(t, r.Add("", p))
		return r, store
	}
	segment := func() schema.AgentMeta {
		return schema.AgentMeta{
			PromptMeta: schema.PromptMeta{
				Name:        "segment",
				Title:       types.Ptr("Segment a transcript"),
				Description: types.Ptr("Segments a transcript into its sections"),
				Input:       json.RawMessage(`{"type":"object","properties":{"transcript":{"type":"string"}},"required":["transcript"]}`),
				Prompt:      types.Ptr("Segment this transcript: {{ .transcript }}"),
			},
			GeneratorMeta: schema.GeneratorMeta{
				Model:  types.Ptr("model"),
				System: types.Ptr("You segment transcripts"),
				Tools:  []string{"current"},
				Opt:    map[string]json.RawMessage{"thinking": json.RawMessage(`false`)},
				Schema: json.RawMessage(`{"type":"object","properties":{"sections":{"type":"array"}}}`),
			},
		}
	}

	t.Run("Create", func(t *testing.T) {
		// An agent is created with its definition, and its model and tools
		// by their canonical names
		r, _ := newRegistry(t)
		agent, err := r.CreateAgent(ctx, segment())
		require.NoError(t, err)
		assert.Equal(t, "segment", agent.Name)
		assert.NotZero(t, agent.Serial)
		assert.False(t, agent.Created.IsZero())
		assert.Equal(t, "scripted:model", types.Value(agent.Model))
		assert.Equal(t, []string{"weather:current"}, agent.Tools)
		assert.Equal(t, "Segment a transcript", types.Value(agent.Title))

		// An agent needs only its name
		agent, err = r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "empty"}})
		require.NoError(t, err)
		assert.Nil(t, agent.Model)

		// And there's one agent with a name
		_, err = r.CreateAgent(ctx, segment())
		assert.ErrorIs(t, err, llm.ErrConflict)
	})

	t.Run("CreateInvalid", func(t *testing.T) {
		// The name must be an identifier, the input a JSON schema of an
		// object, the prompt a Go template, the settings valid, and the
		// model and tools must exist
		r, store := newRegistry(t)
		for name, change := range map[string]func(*schema.AgentMeta){
			"Name":        func(a *schema.AgentMeta) { a.Name = "not a name" },
			"Underscores": func(a *schema.AgentMeta) { a.Name = "my__agent" },
			"Underscore":  func(a *schema.AgentMeta) { a.Name = "agent_" },
			"NoName":      func(a *schema.AgentMeta) { a.Name = "" },
			"Input":       func(a *schema.AgentMeta) { a.Input = json.RawMessage(`{"type":1}`) },
			"InputArray":  func(a *schema.AgentMeta) { a.Input = json.RawMessage(`{"type":"array"}`) },
			"Prompt":      func(a *schema.AgentMeta) { a.Prompt = types.Ptr("{{ .transcript ") },
			"Schema":      func(a *schema.AgentMeta) { a.Schema = json.RawMessage(`{"type":1}`) },
			"Option": func(a *schema.AgentMeta) {
				a.Opt = map[string]json.RawMessage{llm.SystemPromptKey: json.RawMessage(`"x"`)}
			},
		} {
			meta := segment()
			change(&meta)
			_, err := r.CreateAgent(ctx, meta)
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
		meta := segment()
		meta.Model = types.Ptr("missing")
		_, err := r.CreateAgent(ctx, meta)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		meta = segment()
		meta.Tools = []string{"missing"}
		_, err = r.CreateAgent(ctx, meta)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		list, err := store.List(ctx, schema.AgentListRequest{})
		require.NoError(t, err)
		assert.Zero(t, list.Count)
	})

	t.Run("Get", func(t *testing.T) {
		r, _ := newRegistry(t)
		_, err := r.CreateAgent(ctx, segment())
		require.NoError(t, err)
		agent, err := r.GetAgent(ctx, "segment")
		require.NoError(t, err)
		assert.Equal(t, "Segment this transcript: {{ .transcript }}", types.Value(agent.Prompt))
		assert.JSONEq(t, `{"type":"object","properties":{"sections":{"type":"array"}}}`, string(agent.Schema))
		_, err = r.GetAgent(ctx, "missing")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("List", func(t *testing.T) {
		// Agents are listed in order of their names, from an offset, and at
		// most a limit of them
		r, _ := newRegistry(t)
		for _, name := range []string{"summarize", "segment", "translate"} {
			_, err := r.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: name}})
			require.NoError(t, err)
		}
		list, err := r.ListAgents(ctx, schema.AgentListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint(3), list.Count)
		var names []string
		for _, agent := range list.Body {
			names = append(names, agent.Name)
		}
		assert.Equal(t, []string{"segment", "summarize", "translate"}, names)
		page, err := r.ListAgents(ctx, schema.AgentListRequest{Offset: 1, Limit: types.Ptr(uint(1))})
		require.NoError(t, err)
		require.Len(t, page.Body, 1)
		assert.Equal(t, "summarize", page.Body[0].Name)
	})

	t.Run("Update", func(t *testing.T) {
		// What the update sets is changed, and the rest kept, and its name
		// isn't changed
		r, _ := newRegistry(t)
		created, err := r.CreateAgent(ctx, segment())
		require.NoError(t, err)
		agent, err := r.UpdateAgent(ctx, "segment", schema.AgentMeta{
			PromptMeta: schema.PromptMeta{
				Name:        "segment",
				Description: types.Ptr(""),
				Prompt:      types.Ptr("Segment: {{ .transcript }}"),
			},
			GeneratorMeta: schema.GeneratorMeta{
				Model: types.Ptr("other"),
				Opt:   map[string]json.RawMessage{"thinking": json.RawMessage(`null`), "temperature": json.RawMessage(`0.2`)},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "segment", agent.Name)
		assert.Greater(t, agent.Serial, created.Serial)
		assert.Equal(t, "Segment a transcript", types.Value(agent.Title))
		assert.Nil(t, agent.Description)
		assert.Equal(t, "Segment: {{ .transcript }}", types.Value(agent.Prompt))
		assert.Equal(t, "scripted:other", types.Value(agent.Model))
		assert.Equal(t, []string{"weather:current"}, agent.Tools)
		assert.Equal(t, map[string]json.RawMessage{"temperature": json.RawMessage(`0.2`)}, agent.Opt)
		assert.NotEmpty(t, agent.Input)

		// The input, and schema, are cleared with null
		agent, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Input: json.RawMessage(`null`)},
			GeneratorMeta: schema.GeneratorMeta{Schema: json.RawMessage(`null`)},
		})
		require.NoError(t, err)
		assert.Empty(t, agent.Input)
		assert.Empty(t, agent.Schema)
	})

	t.Run("UpdateInvalid", func(t *testing.T) {
		// An update must change something, of an agent which exists, and
		// the definition, as changed, must be valid
		r, _ := newRegistry(t)
		_, err := r.CreateAgent(ctx, segment())
		require.NoError(t, err)
		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "segment"}})
		assert.ErrorIs(t, err, llm.ErrBadParameter, "its name alone changes nothing")
		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "renamed", Title: types.Ptr("Title")}})
		assert.ErrorIs(t, err, llm.ErrBadParameter, "an agent can't be renamed")
		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Prompt: types.Ptr("{{")}})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = r.UpdateAgent(ctx, "segment", schema.AgentMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("missing")}})
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.UpdateAgent(ctx, "missing", schema.AgentMeta{PromptMeta: schema.PromptMeta{Title: types.Ptr("Title")}})
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		r, _ := newRegistry(t)
		_, err := r.CreateAgent(ctx, segment())
		require.NoError(t, err)
		agent, err := r.DeleteAgent(ctx, "segment")
		require.NoError(t, err)
		assert.Equal(t, "segment", agent.Name)
		_, err = r.GetAgent(ctx, "segment")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = r.DeleteAgent(ctx, "segment")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})

	t.Run("Recreated", func(t *testing.T) {
		// An agent which is deleted, and created again, has a serial it
		// didn't have, so a change to it as it was isn't made to the new one
		r, store := newRegistry(t)
		old, err := r.CreateAgent(ctx, segment())
		require.NoError(t, err)
		_, err = r.DeleteAgent(ctx, "segment")
		require.NoError(t, err)
		created, err := r.CreateAgent(ctx, segment())
		require.NoError(t, err)
		assert.NotEqual(t, old.Serial, created.Serial)

		stale := *old
		stale.Title = types.Ptr("Stale")
		_, err = store.Put(ctx, stale)
		assert.ErrorIs(t, err, llm.ErrConflict)
		assert.ErrorIs(t, store.Delete(ctx, "segment", old.Serial), llm.ErrConflict)
		got, err := r.GetAgent(ctx, "segment")
		require.NoError(t, err)
		assert.Equal(t, created.Serial, got.Serial)
		assert.Equal(t, "Segment a transcript", types.Value(got.Title))
	})

	t.Run("NoStore", func(t *testing.T) {
		// A registry without an agent store has no agents
		r, err := registry.New()
		require.NoError(t, err)
		_, err = r.CreateAgent(ctx, segment())
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		_, err = r.GetAgent(ctx, "segment")
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		_, err = r.ListAgents(ctx, schema.AgentListRequest{})
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		_, err = r.UpdateAgent(ctx, "segment", segment())
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
		_, err = r.DeleteAgent(ctx, "segment")
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	})
}
