package httphandler_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// segmentYAML is an agent's definition, in YAML, of an agent which
// segments a transcript
const segmentYAML = `
name: segment
title: Segment a transcript
model: eliza
opt:
  thinking: false
system: You segment transcripts.
input:
  type: object
  properties:
    transcript:
      type: string
prompt: "Segment this transcript: {{ .transcript }}"
schema:
  type: object
  properties:
    sections:
      type: array
`

func Test_Agents(t *testing.T) {
	provider, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}, registry.WithAgentStore(test.NewAgentStore())))

	t.Run("CreateJSON", func(t *testing.T) {
		var agent schema.Agent
		status := do(t, server, http.MethodPost, "llm/agent", map[string]any{
			"name":   "summarize",
			"model":  eliza.ModelName,
			"input":  map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}},
			"prompt": "Summarize this: {{ .text }}",
		}, &agent)
		require.Equal(t, http.StatusCreated, status)
		assert.Equal(t, "summarize", agent.Name)
		assert.NotZero(t, agent.Serial)
		assert.Equal(t, "eliza:eliza-1966-en", types.Value(agent.Model))
	})

	t.Run("CreateYAML", func(t *testing.T) {
		// A definition in YAML, as of a file, is created, with any of its
		// content types
		for i, contentType := range []string{types.ContentTypeYAML, "application/x-yaml", "text/yaml"} {
			name := []string{"segment", "segment2", "segment3"}[i]
			status, body := server.DoWithContentType(t, http.MethodPost, "llm/agent", contentType, strings.Replace(segmentYAML, "name: segment", "name: "+name, 1))
			require.Equal(t, http.StatusCreated, status, string(body))
			var agent schema.Agent
			require.NoError(t, json.Unmarshal(body, &agent))
			assert.Equal(t, name, agent.Name)
			assert.Equal(t, "Segment a transcript", types.Value(agent.Title))
			assert.Equal(t, "You segment transcripts.", types.Value(agent.System))
			assert.JSONEq(t, `{"thinking":false}`, mustJSON(t, agent.Opt))
			assert.JSONEq(t, `{"type":"object","properties":{"sections":{"type":"array"}}}`, string(agent.Schema))
		}
	})

	t.Run("CreateInvalid", func(t *testing.T) {
		// An agent which exists is replaced, or kept, when it's as it is,
		// which isn't a creation, and the YAML has a key which isn't a field,
		// the name isn't valid, or the model isn't found
		status, _ := server.DoWithContentType(t, http.MethodPost, "llm/agent", types.ContentTypeYAML, segmentYAML)
		assert.Equal(t, http.StatusOK, status)
		status, _ = server.DoWithContentType(t, http.MethodPost, "llm/agent", types.ContentTypeYAML, "name: other\nsystem_prompt: Be brief\n")
		assert.Equal(t, http.StatusBadRequest, status)
		status, _ = server.DoWithContentType(t, http.MethodPost, "llm/agent", types.ContentTypeYAML, "name: [other\n")
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/agent", map[string]any{"name": "my__agent"}, nil))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPost, "llm/agent", map[string]any{"name": "other", "model": "missing"}, nil))
		status, _ = server.DoWithContentType(t, http.MethodPost, "llm/agent", types.ContentTypeYAML, "name: big\nsystem: "+strings.Repeat("a", 2<<20)+"\n")
		assert.Equal(t, http.StatusRequestEntityTooLarge, status)
	})
}

func Test_GetAgent(t *testing.T) {
	provider, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}, registry.WithAgentStore(test.NewAgentStore())))
	status, body := server.DoWithContentType(t, http.MethodPost, "llm/agent", types.ContentTypeYAML, segmentYAML)
	require.Equal(t, http.StatusCreated, status, string(body))

	// get requests the agent, accepting the type
	get := func(t *testing.T, name, accept string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+test.Prefix+"/llm/agent/"+name, nil)
		require.NoError(t, err)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp, data
	}

	t.Run("JSON", func(t *testing.T) {
		// The agent, as JSON, by default, with its serial
		for _, accept := range []string{"", types.ContentTypeJSON, types.ContentTypeAny} {
			resp, data := get(t, "segment", accept)
			require.Equal(t, http.StatusOK, resp.StatusCode, accept)
			var agent schema.Agent
			require.NoError(t, json.Unmarshal(data, &agent))
			assert.Equal(t, "segment", agent.Name)
			assert.NotZero(t, agent.Serial)
		}
	})

	t.Run("YAML", func(t *testing.T) {
		// Its definition, as YAML, which can be created again, as it is
		for _, accept := range []string{types.ContentTypeYAML, "text/yaml"} {
			resp, data := get(t, "segment", accept)
			require.Equal(t, http.StatusOK, resp.StatusCode, accept)
			assert.Equal(t, types.ContentTypeYAML, resp.Header.Get(types.ContentTypeHeader))
			assert.True(t, strings.HasPrefix(string(data), "name: segment\n"), string(data))
			assert.NotContains(t, string(data), "serial")
			meta, err := schema.ParseAgentMeta(data)
			require.NoError(t, err)
			assert.Equal(t, "eliza:eliza-1966-en", types.Value(meta.Model))
		}
		resp, data := get(t, "segment", types.ContentTypeYAML)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		status, body := server.DoWithContentType(t, http.MethodPost, "llm/agent", types.ContentTypeYAML, strings.Replace(string(data), "name: segment", "name: copy", 1))
		assert.Equal(t, http.StatusCreated, status, string(body))
	})

	t.Run("Errors", func(t *testing.T) {
		resp, _ := get(t, "missing", "")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp, _ = get(t, "segment", "text/html")
		assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
	})
}

func Test_ListUpdateDeleteAgents(t *testing.T) {
	provider, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}, registry.WithAgentStore(test.NewAgentStore())))
	for _, name := range []string{"summarize", "segment", "translate"} {
		require.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/agent", map[string]any{"name": name, "title": "Title of " + name}, nil))
	}

	t.Run("List", func(t *testing.T) {
		// Agents are listed in order of their names, from an offset, and at
		// most a limit of them
		var list schema.AgentList
		require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/agent", nil, &list))
		assert.Equal(t, uint(3), list.Count)
		require.Len(t, list.Body, 3)
		assert.Equal(t, "segment", list.Body[0].Name)
		require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/agent?offset=1&limit=1", nil, &list))
		assert.Equal(t, uint(3), list.Count)
		require.Len(t, list.Body, 1)
		assert.Equal(t, "summarize", list.Body[0].Name)
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodGet, "llm/agent?limit=x", nil, nil))
	})

	t.Run("UpdateJSON", func(t *testing.T) {
		// What's set is changed, and the rest kept
		var agent schema.Agent
		require.Equal(t, http.StatusOK, do(t, server, http.MethodPatch, "llm/agent/segment", map[string]any{
			"description": "Segments a transcript",
			"opt":         map[string]any{"thinking": false},
		}, &agent))
		assert.Equal(t, "segment", agent.Name)
		assert.Equal(t, "Title of segment", types.Value(agent.Title))
		assert.Equal(t, "Segments a transcript", types.Value(agent.Description))
		assert.JSONEq(t, `{"thinking":false}`, mustJSON(t, agent.Opt))
	})

	t.Run("UpdateYAML", func(t *testing.T) {
		// The changes, as YAML, and the agent, as YAML, when it's accepted
		req, err := http.NewRequest(http.MethodPatch, server.URL+test.Prefix+"/llm/agent/segment", strings.NewReader("title: Segment a transcript\nopt:\n  thinking: null\n"))
		require.NoError(t, err)
		req.Header.Set(types.ContentTypeHeader, types.ContentTypeYAML)
		req.Header.Set("Accept", types.ContentTypeYAML)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
		assert.Equal(t, types.ContentTypeYAML, resp.Header.Get(types.ContentTypeHeader))
		assert.Equal(t, "name: segment\ntitle: Segment a transcript\ndescription: Segments a transcript\n", string(data))
	})

	t.Run("UpdateInvalid", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPatch, "llm/agent/segment", map[string]any{}, nil))
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPatch, "llm/agent/segment", map[string]any{"prompt": "{{"}, nil))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPatch, "llm/agent/missing", map[string]any{"title": "Title"}, nil))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPatch, "llm/agent/segment", map[string]any{"model": "missing"}, nil))
		status, _ := server.DoWithContentType(t, http.MethodPatch, "llm/agent/segment", types.ContentTypeYAML, "system_prompt: Be brief\n")
		assert.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("Delete", func(t *testing.T) {
		var agent schema.Agent
		require.Equal(t, http.StatusOK, do(t, server, http.MethodDelete, "llm/agent/translate", nil, &agent))
		assert.Equal(t, "translate", agent.Name)
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/agent/translate", nil, nil))
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodDelete, "llm/agent/translate", nil, nil))
	})
}

func Test_AgentsNoStore(t *testing.T) {
	// A server without an agent store has no agents' API
	server, _ := newServer(t)
	assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPost, "llm/agent", map[string]any{"name": "segment"}, nil))
	_, documented := server.Spec().Paths.MapOfPathItemValues["/llm/agent"]
	assert.False(t, documented)
}

// mustJSON returns the value as JSON
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func Test_BuiltinAgent(t *testing.T) {
	// A client can't make an agent built in, and a built-in agent, which the
	// server created, can't be replaced, changed or deleted, which is 403
	provider, err := eliza.New()
	require.NoError(t, err)
	agents := test.NewAgentStore()
	r := test.NewRegistry(t, []llm.Provider{provider}, registry.WithAgentStore(agents))
	server := test.NewServer(t, r)

	var agent schema.Agent
	status := do(t, server, http.MethodPost, "llm/agent", map[string]any{"name": "mine", "prompt": "Hello", "builtin": true}, &agent)
	require.Equal(t, http.StatusCreated, status)
	assert.False(t, types.Value(agent.Builtin), "a client can't make an agent built in")

	_, _, err = r.CreateAgent(t.Context(), schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "shipped", Prompt: types.Ptr("Hello"), Builtin: types.Ptr(true)}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, do(t, server, http.MethodPost, "llm/agent", map[string]any{"name": "shipped", "prompt": "Changed"}, nil))
	assert.Equal(t, http.StatusForbidden, do(t, server, http.MethodPatch, "llm/agent/shipped", map[string]any{"prompt": "Changed"}, nil))
	assert.Equal(t, http.StatusForbidden, do(t, server, http.MethodDelete, "llm/agent/shipped", nil, nil))
	var got schema.Agent
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/agent/shipped", nil, &got))
	assert.True(t, types.Value(got.Builtin))

	// It's in the builtin namespace, so it's found by that, and not by the
	// agent namespace's
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/agent/builtin:shipped", nil, &got))
	assert.Equal(t, "shipped", got.Name)
	assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/agent/agent:shipped", nil, nil))
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/agent/builtin%3Ashipped", nil, &got), "as a client escapes it")
	assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/agent/builtin:mine", nil, nil))

	// Its model, and only its model, can be changed
	require.Equal(t, http.StatusOK, do(t, server, http.MethodPatch, "llm/agent/builtin:shipped", map[string]any{"model": eliza.ModelName}, &got))
	require.NotNil(t, got.Model)
	model := *got.Model
	assert.Contains(t, model, eliza.ModelName)
	assert.Equal(t, http.StatusForbidden, do(t, server, http.MethodPatch, "llm/agent/shipped", map[string]any{"model": eliza.ModelName, "prompt": "Changed"}, nil))

	// And it's kept, as the server creates it again, with a new prompt
	_, _, err = r.CreateAgent(t.Context(), schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "shipped", Prompt: types.Ptr("Hello again"), Builtin: types.Ptr(true)}})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/agent/shipped", nil, &got))
	assert.Equal(t, "Hello again", types.Value(got.Prompt))
	assert.Equal(t, model, types.Value(got.Model), "the model chosen for it is kept")

	// An agent which an older server stored as an ordinary agent, with a
	// model, keeps it, as the server creates it again, built in
	require.Equal(t, http.StatusCreated, do(t, server, http.MethodPost, "llm/agent", map[string]any{"name": "legacy", "prompt": "Hello", "model": eliza.ModelName}, nil))
	_, _, err = r.CreateAgent(t.Context(), schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "legacy", Prompt: types.Ptr("Hello"), Builtin: types.Ptr(true)}})
	require.NoError(t, err)
	var legacy schema.Agent
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/agent/builtin:legacy", nil, &legacy))
	assert.True(t, types.Value(legacy.Builtin))
	assert.Equal(t, model, types.Value(legacy.Model), "the model it had is kept")

	// Its model can be cleared, so it's the default again
	var cleared schema.Agent
	require.Equal(t, http.StatusOK, do(t, server, http.MethodPatch, "llm/agent/shipped", map[string]any{"model": ""}, &cleared))
	assert.Nil(t, cleared.Model)
}
