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
	assert.Equal(t, []string{"translate"}, names)
}
