package httpclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	registry "github.com/mutablelogic/go-llm/registry"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newAgentClient returns a client of a test server of a registry with an
// agent store, whose requests and responses are checked against the API's
// OpenAPI specification when the test ends
func newAgentClient(t *testing.T) *httpclient.Client {
	t.Helper()
	provider, err := eliza.New(eliza.WithTools())
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}, registry.WithAgentStore(test.NewAgentStore())))
	client, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	return client
}

func Test_Agents(t *testing.T) {
	ctx := context.Background()
	client := newAgentClient(t)

	t.Run("Create", func(t *testing.T) {
		agent, err := client.CreateAgent(ctx, schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Name: "summarize", Input: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`), Prompt: types.Ptr("Summarize: {{ .text }}")},
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(eliza.ModelName)},
		})
		require.NoError(t, err)
		assert.Equal(t, "summarize", agent.Name)
		assert.Equal(t, "eliza:eliza-1966-en", types.Value(agent.Model))

		// An agent with the name is replaced, when it's changed
		replaced, err := client.CreateAgent(ctx, schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "summarize"}})
		require.NoError(t, err)
		assert.Nil(t, replaced.Model)
		assert.Greater(t, replaced.Serial, agent.Serial)
	})

	t.Run("CreateYAML", func(t *testing.T) {
		agent, err := client.WriteAgentYAML(ctx, strings.NewReader("name: segment\ntitle: Segment a transcript\nsystem: |\n  You segment transcripts.\nopt:\n  thinking: false\n"))
		require.NoError(t, err)
		assert.Equal(t, "segment", agent.Name)
		assert.Equal(t, "You segment transcripts.\n", types.Value(agent.System))

		_, err = client.WriteAgentYAML(ctx, strings.NewReader("name: other\nsystem_prompt: Be brief\n"))
		assert.Equal(t, http.StatusBadRequest, status(t, err))
	})

	t.Run("Get", func(t *testing.T) {
		agent, err := client.GetAgent(ctx, "segment")
		require.NoError(t, err)
		assert.Equal(t, "Segment a transcript", types.Value(agent.Title))
		assert.NotZero(t, agent.Serial)
		_, err = client.GetAgent(ctx, "missing")
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})

	t.Run("GetYAML", func(t *testing.T) {
		// The definition, as YAML, which is created again, as it is
		var buf bytes.Buffer
		require.NoError(t, client.ReadAgentYAML(ctx, "segment", &buf))
		data := buf.String()
		assert.True(t, strings.HasPrefix(data, "name: segment\ntitle: Segment a transcript\n"), data)
		assert.Contains(t, data, "system: |\n  You segment transcripts.\n")
		copied, err := client.WriteAgentYAML(ctx, strings.NewReader(strings.Replace(data, "name: segment", "name: copy", 1)))
		require.NoError(t, err)
		assert.Equal(t, "copy", copied.Name)
		err = client.ReadAgentYAML(ctx, "missing", &bytes.Buffer{})
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})

	t.Run("List", func(t *testing.T) {
		list, err := client.ListAgents(ctx, schema.AgentListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint(3), list.Count)
		page, err := client.ListAgents(ctx, schema.AgentListRequest{Offset: 1, Limit: types.Ptr(uint(1))})
		require.NoError(t, err)
		assert.Equal(t, uint(3), page.Count)
		assert.Len(t, page.Body, 1)
	})

	t.Run("Update", func(t *testing.T) {
		agent, err := client.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Description: types.Ptr("Segments a transcript")}})
		require.NoError(t, err)
		assert.Equal(t, "Segment a transcript", types.Value(agent.Title))
		assert.Equal(t, "Segments a transcript", types.Value(agent.Description))

		agent, err = client.UpdateAgent(ctx, "segment", schema.AgentMeta{
			PromptMeta:    schema.PromptMeta{Description: types.Ptr("")},
			GeneratorMeta: schema.GeneratorMeta{Opt: map[string]json.RawMessage{"thinking": json.RawMessage(`null`)}},
		})
		require.NoError(t, err)
		assert.Nil(t, agent.Description)
		assert.Empty(t, agent.Opt)

		_, err = client.UpdateAgent(ctx, "segment", schema.AgentMeta{})
		assert.Equal(t, http.StatusBadRequest, status(t, err))
		_, err = client.UpdateAgent(ctx, "segment", schema.AgentMeta{PromptMeta: schema.PromptMeta{Name: "renamed", Title: types.Ptr("Title")}})
		assert.Equal(t, http.StatusBadRequest, status(t, err))
		_, err = client.UpdateAgent(ctx, "missing", schema.AgentMeta{PromptMeta: schema.PromptMeta{Title: types.Ptr("Title")}})
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})

	t.Run("Delete", func(t *testing.T) {
		agent, err := client.DeleteAgent(ctx, "copy")
		require.NoError(t, err)
		assert.Equal(t, "copy", agent.Name)
		_, err = client.DeleteAgent(ctx, "copy")
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})
}
