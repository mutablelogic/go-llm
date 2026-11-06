package agents_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Packages
	agents "github.com/mutablelogic/go-llm/agents"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// examples are an input of each agent, which its prompt is executed with
var examples = map[string]string{
	"translate": `{"text":"Guten Morgen"}`,
	"event":     `{"title":"Morning briefing","prompt":"Give David the weather in Berlin today","context":"Now: Friday, 6 November 2026, 08:00, Europe/Berlin"}`,
}

func Test_Agents(t *testing.T) {
	// Each agent is valid, named as its file, and its prompt is executed with
	// its example input
	files, err := filepath.Glob("*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			require.NoError(t, err)
			meta, err := schema.ParseAgentMeta(data)
			require.NoError(t, err)
			assert.Equal(t, strings.TrimSuffix(file, ".yaml"), meta.Name)
			assert.NotEmpty(t, types.Value(meta.Description))
			_, err = meta.JSONSchema()
			require.NoError(t, err)
			input, ok := examples[meta.Name]
			require.True(t, ok, "an example input of %q", meta.Name)
			message, err := meta.Message(json.RawMessage(input))
			require.NoError(t, err)
			assert.NotEmpty(t, message)
		})
	}
}

func Test_Translate(t *testing.T) {
	// The text is translated into English, unless another language is given,
	// and the language it's in is said, when it's known, and the reply has
	// the translation, and the two-letter code of the text's language
	data, err := os.ReadFile("translate.yaml")
	require.NoError(t, err)
	meta, err := schema.ParseAgentMeta(data)
	require.NoError(t, err)
	for input, want := range map[string]string{
		`{"text":"Guten Morgen"}`:                                     "Translate this text into English:\n\nGuten Morgen\n",
		`{"text":"Guten Morgen","language":"French"}`:                 "Translate this text into French:\n\nGuten Morgen\n",
		`{"text":"Guten Morgen","language":"French","from":"German"}`: "Translate this text into French, from German:\n\nGuten Morgen\n",
	} {
		message, err := meta.Message(json.RawMessage(input))
		require.NoError(t, err, input)
		assert.Equal(t, want, message, input)
	}
	_, err = meta.Message(json.RawMessage(`{"language":"French"}`))
	assert.Error(t, err, "the text is required")

	reply, err := meta.JSONSchema()
	require.NoError(t, err)
	require.NotNil(t, reply)
	assert.NoError(t, reply.Validate(json.RawMessage(`{"text":"Good morning","language":"de"}`)))
	assert.Error(t, reply.Validate(json.RawMessage(`{"text":"Good morning","language":"German"}`)), "a two-letter code")
	assert.Error(t, reply.Validate(json.RawMessage(`{"text":"Good morning"}`)), "with the language")
}

func Test_All(t *testing.T) {
	// The agents are embedded, one of each file
	all, err := agents.All()
	require.NoError(t, err)
	files, err := filepath.Glob("*.yaml")
	require.NoError(t, err)
	require.Len(t, all, len(files))
	var names []string
	for _, meta := range all {
		names = append(names, meta.Name)
	}
	assert.Equal(t, []string{"event", "translate"}, names)
}

func Test_Event(t *testing.T) {
	// The event's title, and its instruction, how late it is, and what the
	// system says, in a block of its own, first, when they're given, and the
	// reply has the message, and its importance
	data, err := os.ReadFile("event.yaml")
	require.NoError(t, err)
	meta, err := schema.ParseAgentMeta(data)
	require.NoError(t, err)
	for input, want := range map[string]string{
		`{"title":"Call","prompt":"Remind David to call his mother","context":"Now: 18:00"}`:                                  "<system-reminder>\nNow: 18:00\n</system-reminder>\nThe event \"Call\" has fired. Its instruction is:\n\nRemind David to call his mother\n",
		`{"title":"Call","prompt":"Remind David to call his mother","late":"3h12m late","context":"Now: 18:00\nName: David"}`: "<system-reminder>\nNow: 18:00\nName: David\n</system-reminder>\nThe event \"Call\" has fired, 3h12m late. Its instruction is:\n\nRemind David to call his mother\n",
	} {
		message, err := meta.Message(json.RawMessage(input))
		require.NoError(t, err, input)
		assert.Equal(t, want, message, input)
	}
	_, err = meta.Message(json.RawMessage(`{"title":"Call","context":"Now: 18:00"}`))
	assert.Error(t, err, "the prompt is required")
	_, err = meta.Message(json.RawMessage(`{"title":"Call","prompt":"Remind David"}`))
	assert.Error(t, err, "the context is required, so the time is known")
	assert.True(t, types.Value(meta.Internal), "only the server runs it")
	assert.Empty(t, meta.Tools)
	assert.Nil(t, meta.Model)

	reply, err := meta.JSONSchema()
	require.NoError(t, err)
	require.NotNil(t, reply)
}
