package schema_test

import (
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_SessionUsage(t *testing.T) {
	// The usage's tokens are alongside those of the conversation
	usage := schema.SessionUsage{
		Usage: llm.Usage{InputTokens: 10, OutputTokens: 5, CachedTokens: 2},
		Input: 1200, Output: 300, Overhead: 150,
	}
	data, err := json.Marshal(usage)
	require.NoError(t, err)
	assert.JSONEq(t, `{"input_tokens":10,"output_tokens":5,"cached_tokens":2,"input":1200,"output":300,"overhead":150}`, string(data))

	var decoded schema.SessionUsage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, usage, decoded)
}

func Test_SessionMetaMerge(t *testing.T) {
	meta := func(g schema.GeneratorMeta) schema.SessionMeta { return schema.SessionMeta{GeneratorMeta: g} }
	opt := func(kv ...string) map[string]json.RawMessage {
		result := make(map[string]json.RawMessage, len(kv)/2)
		for i := 0; i < len(kv); i += 2 {
			result[kv[i]] = json.RawMessage(kv[i+1])
		}
		return result
	}
	existing := meta(schema.GeneratorMeta{
		Model:  types.Ptr("ollama:llama3.2"),
		System: types.Ptr("Be brief"),
		Tools:  []string{"weather:*"},
		Opt:    opt("temperature", `0.5`, "thinking", `false`),
		Schema: json.RawMessage(`{"type":"string"}`),
	})

	for name, test := range map[string]struct {
		update schema.SessionMeta
		want   schema.SessionMeta
	}{
		"Nothing": {meta(schema.GeneratorMeta{}), existing},
		"Model": {meta(schema.GeneratorMeta{Model: types.Ptr("openai:gpt-4o")}), meta(schema.GeneratorMeta{
			Model: types.Ptr("openai:gpt-4o"), System: types.Ptr("Be brief"), Tools: []string{"weather:*"}, Opt: opt("temperature", `0.5`, "thinking", `false`), Schema: json.RawMessage(`{"type":"string"}`),
		})},
		"System": {meta(schema.GeneratorMeta{System: types.Ptr("Be thorough")}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), System: types.Ptr("Be thorough"), Tools: []string{"weather:*"}, Opt: opt("temperature", `0.5`, "thinking", `false`), Schema: json.RawMessage(`{"type":"string"}`),
		})},
		"ClearModel": {meta(schema.GeneratorMeta{Model: types.Ptr("")}), meta(schema.GeneratorMeta{
			System: types.Ptr("Be brief"), Tools: []string{"weather:*"}, Opt: opt("temperature", `0.5`, "thinking", `false`), Schema: json.RawMessage(`{"type":"string"}`),
		})},
		"ClearSystem": {meta(schema.GeneratorMeta{System: types.Ptr("")}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), Tools: []string{"weather:*"}, Opt: opt("temperature", `0.5`, "thinking", `false`), Schema: json.RawMessage(`{"type":"string"}`),
		})},
		"BlankSystem": {meta(schema.GeneratorMeta{System: types.Ptr("  ")}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), Tools: []string{"weather:*"}, Opt: opt("temperature", `0.5`, "thinking", `false`), Schema: json.RawMessage(`{"type":"string"}`),
		})},
		"Tools": {meta(schema.GeneratorMeta{Tools: []string{"memory:*", "news:*"}}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), System: types.Ptr("Be brief"), Tools: []string{"memory:*", "news:*"}, Opt: opt("temperature", `0.5`, "thinking", `false`), Schema: json.RawMessage(`{"type":"string"}`),
		})},
		"ClearTools": {meta(schema.GeneratorMeta{Tools: []string{}}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), System: types.Ptr("Be brief"), Opt: opt("temperature", `0.5`, "thinking", `false`), Schema: json.RawMessage(`{"type":"string"}`),
		})},
		"Schema": {meta(schema.GeneratorMeta{Schema: json.RawMessage(`{"type":"number"}`)}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), System: types.Ptr("Be brief"), Tools: []string{"weather:*"}, Opt: opt("temperature", `0.5`, "thinking", `false`), Schema: json.RawMessage(`{"type":"number"}`),
		})},
		"ClearSchema": {meta(schema.GeneratorMeta{Schema: json.RawMessage(`null`)}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), System: types.Ptr("Be brief"), Tools: []string{"weather:*"}, Opt: opt("temperature", `0.5`, "thinking", `false`),
		})},
		"Options": {meta(schema.GeneratorMeta{Opt: opt("temperature", `0.9`, "context_window", `32768`, "thinking", `null`)}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), System: types.Ptr("Be brief"), Tools: []string{"weather:*"}, Opt: opt("temperature", `0.9`, "context_window", `32768`), Schema: json.RawMessage(`{"type":"string"}`),
		})},
		"RemoveAllOptions": {meta(schema.GeneratorMeta{Opt: opt("temperature", `null`, "thinking", ` null `)}), meta(schema.GeneratorMeta{
			Model: types.Ptr("ollama:llama3.2"), System: types.Ptr("Be brief"), Tools: []string{"weather:*"}, Schema: json.RawMessage(`{"type":"string"}`),
		})},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.want, existing.Merge(test.update))
		})
	}

	t.Run("Unchanged", func(t *testing.T) {
		// The settings merged into aren't changed
		before := existing.Opt["temperature"]
		_ = existing.Merge(meta(schema.GeneratorMeta{Opt: opt("temperature", `0.1`, "thinking", `null`)}))
		assert.Equal(t, json.RawMessage(`0.5`), before)
		assert.Len(t, existing.Opt, 2)
	})

	t.Run("FromJSON", func(t *testing.T) {
		// An update as JSON clears the schema with null, the tools with an
		// empty list, so there are none, and removes an option with null, but what it doesn't
		// have, such as the system prompt, is kept
		var update schema.SessionMeta
		require.NoError(t, json.Unmarshal([]byte(`{"schema":null,"tools":[],"opt":{"thinking":null}}`), &update))
		merged := existing.Merge(update)
		assert.Equal(t, "ollama:llama3.2", types.Value(merged.Model))
		assert.Equal(t, "Be brief", types.Value(merged.System))
		assert.Empty(t, merged.Schema)
		assert.Nil(t, merged.Tools)
		assert.Equal(t, opt("temperature", `0.5`), merged.Opt)
	})
}
