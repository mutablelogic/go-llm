package schema_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ParseAgentMeta(t *testing.T) {
	t.Run("YAML", func(t *testing.T) {
		// A definition in YAML, such as go-nanoradio's segment agent, has the
		// names its JSON has, and its input, options and schema are JSON
		meta, err := schema.ParseAgentMeta([]byte(`
# The agent which segments a transcript
name: segment
title: Segment a transcript
description: |
  Segments the transcript of an episode into its sections.
model: qwen3.8:latest
opt:
  thinking: false
  context_window: 32768
system: |
  You segment podcast transcripts.
tools:
  - weatherapi:*
input:
  type: object
  properties:
    transcript:
      type: string
  required:
    - transcript
prompt: |
  Segment this transcript.

  {{ .transcript }}
schema:
  type: object
  properties:
    sections:
      type: array
`))
		require.NoError(t, err)
		assert.Equal(t, "segment", meta.Name)
		assert.Equal(t, "Segment a transcript", types.Value(meta.Title))
		assert.Equal(t, "Segments the transcript of an episode into its sections.\n", types.Value(meta.Description))
		assert.Equal(t, "qwen3.8:latest", types.Value(meta.Model))
		assert.Equal(t, "You segment podcast transcripts.\n", types.Value(meta.System))
		assert.Equal(t, []string{"weatherapi:*"}, meta.Tools)
		assert.JSONEq(t, `false`, string(meta.Opt["thinking"]))
		assert.JSONEq(t, `32768`, string(meta.Opt["context_window"]))
		assert.JSONEq(t, `{"type":"object","properties":{"transcript":{"type":"string"}},"required":["transcript"]}`, string(meta.Input))
		assert.Equal(t, `{"type":"object","properties":{"transcript":{"type":"string"}},"required":["transcript"]}`, string(meta.Input), "in the order it was written")
		assert.JSONEq(t, `{"type":"object","properties":{"sections":{"type":"array"}}}`, string(meta.Schema))
		assert.Contains(t, types.Value(meta.Prompt), "{{ .transcript }}")
		input, err := meta.InputSchema()
		require.NoError(t, err)
		assert.NotNil(t, input)
		_, err = meta.Template()
		require.NoError(t, err)
	})

	t.Run("JSON", func(t *testing.T) {
		// JSON is YAML too
		meta, err := schema.ParseAgentMeta([]byte(`{"name":"summarize","prompt":"Summarize {{ .text }}","opt":{"temperature":0.2}}`))
		require.NoError(t, err)
		assert.Equal(t, "summarize", meta.Name)
		assert.JSONEq(t, `0.2`, string(meta.Opt["temperature"]))
	})

	t.Run("Invalid", func(t *testing.T) {
		// A key which isn't one of its names, such as the segment agent's
		// earlier system_prompt, options and format, isn't valid, nor is
		// what isn't a mapping, or YAML
		for name, data := range map[string]string{
			"SystemPrompt": "name: segment\nsystem_prompt: Be brief\n",
			"Options":      "name: segment\noptions:\n  thinking: false\n",
			"Format":       "name: segment\nformat:\n  type: object\n",
			"List":         "- name: segment\n",
			"Text":         "segment",
			"YAML":         "name: [segment\n",
		} {
			_, err := schema.ParseAgentMeta([]byte(data))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
	})
}

func Test_AgentMetaYAML(t *testing.T) {
	// The definition is YAML, as a file of it is, in the order of its JSON,
	// with text of more than one line as a literal block, which is parsed
	// as it was
	meta, err := schema.ParseAgentMeta([]byte(`{
		"name": "segment",
		"title": "Segment: a transcript",
		"input": {"type": "object", "properties": {"transcript": {"type": "string"}}},
		"prompt": "Segment this transcript.\n\n{{ .transcript }}\n",
		"model": "ollama:qwen3.8:latest",
		"system": "true",
		"tools": ["weatherapi:*"],
		"opt": {"thinking": false, "context_window": 32768},
		"schema": {"type": "object"}
	}`))
	require.NoError(t, err)
	data, err := meta.YAML()
	require.NoError(t, err)
	text := string(data)
	assert.True(t, strings.HasPrefix(text, "name: segment\ntitle: "), text)
	assert.Contains(t, text, "prompt: |\n  Segment this transcript.\n\n  {{ .transcript }}\n")
	assert.Contains(t, text, "system: \"true\"\n", "text which isn't plain is quoted")
	assert.Contains(t, text, "input:\n  type: object\n  properties:\n", "in the order it was written")
	assert.Less(t, strings.Index(text, "input:"), strings.Index(text, "prompt:"))
	assert.Less(t, strings.Index(text, "prompt:"), strings.Index(text, "model:"))
	assert.NotContains(t, text, "{\"")

	again, err := schema.ParseAgentMeta(data)
	require.NoError(t, err)
	assert.Equal(t, meta.Name, again.Name)
	assert.Equal(t, meta.Title, again.Title)
	assert.Equal(t, meta.Prompt, again.Prompt)
	assert.Equal(t, meta.System, again.System)
	assert.Equal(t, meta.Tools, again.Tools)
	assert.JSONEq(t, string(meta.Input), string(again.Input))
	assert.JSONEq(t, string(meta.Schema), string(again.Schema))
	assert.JSONEq(t, `{"thinking":false,"context_window":32768}`, mustJSON(t, again.Opt))
}

// mustJSON returns the value as JSON
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func Test_AgentUpdated(t *testing.T) {
	// The table's last column is when the agent was last updated: when it
	// was created, or modified, whichever is later
	created := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	modified := created.Add(time.Hour)
	assert.Equal(t, "Updated", schema.Agent{}.Header()[4])
	assert.Equal(t, modified.Local().Format(time.DateTime), schema.Agent{Created: created, Modified: modified}.Cell(4))
	assert.Equal(t, created.Local().Format(time.DateTime), schema.Agent{Created: created}.Cell(4))
	assert.Equal(t, created.Local().Format(time.DateTime), schema.Agent{Created: created, Modified: created.Add(-time.Hour)}.Cell(4))
	assert.Equal(t, "", schema.Agent{}.Cell(4))
}
