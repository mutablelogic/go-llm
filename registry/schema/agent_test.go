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
			"Loop":         "name: loop\ninput: &loop [*loop]\n",
			"Alias":        "name: segment\ntitle: &title Segment\ndescription: *title\n",
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

func Test_AgentMessage(t *testing.T) {
	prompt := schema.PromptMeta{
		Name:   "segment",
		Input:  json.RawMessage(`{"type":"object","properties":{"transcript":{"type":"string"},"minutes":{"type":"number"},"tags":{"type":"array"}},"required":["transcript"]}`),
		Prompt: types.Ptr("Segment this transcript, of {{ .minutes }} minutes, tagged {{ json .tags }}:\n\n{{ .transcript }}"),
	}

	t.Run("Message", func(t *testing.T) {
		// The prompt is executed with the input, whose numbers are as they
		// were written, and json writes a value as JSON
		message, err := prompt.Message(json.RawMessage(`{"transcript":"Hello","minutes":37.5,"tags":["news","de"]}`))
		require.NoError(t, err)
		assert.Equal(t, "Segment this transcript, of 37.5 minutes, tagged [\"news\",\"de\"]:\n\nHello", message)
	})

	t.Run("Invalid", func(t *testing.T) {
		// The input is validated by the input schema, and must be an object
		for name, input := range map[string]string{
			"Required": `{"minutes":1,"tags":[]}`,
			"Type":     `{"transcript":1,"minutes":1,"tags":[]}`,
			"Array":    `["transcript"]`,
			"JSON":     `{"transcript":`,
			"Trailing": `{"transcript":"Hello","minutes":1,"tags":[]} trailing`,
			"Two":      `{"transcript":"Hello","minutes":1,"tags":[]} {}`,
		} {
			_, err := prompt.Message(json.RawMessage(input))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
		assert.NoError(t, prompt.Validate(json.RawMessage(`{"transcript":"Hello"}`)), "valid, though the prompt uses more")
	})

	t.Run("Optional", func(t *testing.T) {
		// A property which the input hasn't is its default, or else the empty
		// value of its type, so a prompt can test it, or write it
		p := schema.PromptMeta{
			Name:   "segment",
			Input:  json.RawMessage(`{"type":"object","properties":{"transcript":{"type":"string"},"pauses":{"type":"boolean"},"min_gap":{"type":"number","default":2.5},"speaker":{"type":"string"},"tags":{"type":"array"},"times":{"type":["integer","null"]}}}`),
			Prompt: types.Ptr(`{{ .transcript }}{{ if .pauses }} with pauses{{ end }}, gap {{ .min_gap }}, speaker [{{ .speaker }}], tags {{ json .tags }}, {{ .times }}`),
		}
		message, err := p.Message(json.RawMessage(`{"transcript":"Hello"}`))
		require.NoError(t, err)
		assert.Equal(t, "Hello, gap 2.5, speaker [], tags [], 0", message)
		message, err = p.Message(json.RawMessage(`{"transcript":"Hello","pauses":true,"min_gap":1}`))
		require.NoError(t, err)
		assert.Equal(t, "Hello with pauses, gap 1, speaker [], tags [], 0", message)

		// A nullable type is its type, whichever order its types are in
		p.Input = json.RawMessage(`{"type":"object","properties":{"a":{"type":["null","integer"]},"b":{"type":["integer","null"]},"c":{"type":["null","boolean"]}}}`)
		p.Prompt = types.Ptr(`{{ .a }} {{ .b }} {{ .c }}`)
		message, err = p.Message(json.RawMessage(`{}`))
		require.NoError(t, err)
		assert.Equal(t, "0 0 false", message)

		// Zero is false, whether it's omitted, the default, or given, however
		// it's written, in lists too, and other numbers are true, and written
		// as they were, without losing their precision
		p.Input = json.RawMessage(`{"type":"object","properties":{"count":{"type":"integer"},"limit":{"type":"number","default":0},"list":{"type":"array"}}}`)
		p.Prompt = types.Ptr(`{{ if .count }}count {{ .count }}{{ else }}none{{ end }}, {{ if .limit }}limit{{ else }}no limit{{ end }}, {{ range .list }}{{ if . }}{{ . }}{{ else }}zero{{ end }} {{ end }}`)
		for input, want := range map[string]string{
			`{}`:                             "none, no limit, ",
			`{"count":0,"limit":0.0}`:        "none, no limit, ",
			`{"count":3,"limit":1.5}`:        "count 3, limit, ",
			`{"count":12345678901234567890}`: "count 12345678901234567890, no limit, ",
			`{"list":[0,2.50,0.0,1e3]}`:      "none, no limit, zero 2.50 zero 1e3 ",
		} {
			message, err := p.Message(json.RawMessage(input))
			require.NoError(t, err, input)
			assert.Equal(t, want, message, input)
		}

		// A field which the schema hasn't, such as a misspelt one, is still
		// an error, which is returned
		p.Input = json.RawMessage(`{"type":"object","properties":{"transcript":{"type":"string"}}}`)
		p.Prompt = types.Ptr(`{{ .transcript }} {{ .paused }}`)
		_, err = p.Message(json.RawMessage(`{"transcript":"Hello"}`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.ErrorContains(t, err, `"paused"`)
	})

	t.Run("Resources", func(t *testing.T) {
		// A property of format resource, which is a resource's URI, is its
		// text, which the function reads, and otherwise as it is, as it is
		// without the function
		p := schema.PromptMeta{
			Name:   "segment",
			Input:  json.RawMessage(`{"type":"object","properties":{"transcript":{"type":"string","format":"resource"},"note":{"type":"string"}}}`),
			Prompt: types.Ptr(`{{ .transcript }} / {{ .note }}`),
		}
		read := func(uri string) (string, error) {
			if uri == "resource://abc" {
				return "Hello, as it is", nil
			}
			return "", llm.ErrNotFound.Withf("no %s", uri)
		}
		message, err := p.MessageWith(json.RawMessage(`{"transcript":" resource://abc ","note":"resource://abc"}`), read)
		require.NoError(t, err)
		assert.Equal(t, "Hello, as it is / resource://abc", message)
		message, err = p.MessageWith(json.RawMessage(`{"transcript":"Some text"}`), read)
		require.NoError(t, err)
		assert.Equal(t, "Some text / ", message)
		message, err = p.Message(json.RawMessage(`{"transcript":"resource://abc"}`))
		require.NoError(t, err)
		assert.Equal(t, "resource://abc / ", message)
		_, err = p.MessageWith(json.RawMessage(`{"transcript":"resource://xyz"}`), read)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.ErrorContains(t, err, `input "transcript"`)
	})

	t.Run("NoSchema", func(t *testing.T) {
		// Without an input schema, the input is any object, and no input is
		// an empty one
		p := schema.PromptMeta{Name: "hello", Prompt: types.Ptr("Say hello")}
		message, err := p.Message(nil)
		require.NoError(t, err)
		assert.Equal(t, "Say hello", message)
		assert.NoError(t, p.Validate(json.RawMessage(`{"any":"thing"}`)))
		assert.ErrorIs(t, p.Validate(json.RawMessage(`"text"`)), llm.ErrBadParameter)
	})

	t.Run("NoPrompt", func(t *testing.T) {
		// Without a prompt, the message is the input, as JSON, and there must
		// be one
		p := schema.PromptMeta{Name: "echo"}
		message, err := p.Message(json.RawMessage(`{"text":"Hello"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"text":"Hello"}`, message)
		_, err = p.Message(nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}
