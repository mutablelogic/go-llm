package cmd

import (
	"encoding/json"
	"testing"
	"uuid"

	// Packages
	kong "github.com/alecthomas/kong"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// parseSession parses the arguments of the session commands, and returns
// them
func parseSession(t *testing.T, args ...string) *SessionCommands {
	t.Helper()
	var cmds SessionCommands
	parser, err := kong.New(&cmds)
	require.NoError(t, err)
	_, err = parser.Parse(args)
	require.NoError(t, err)
	return &cmds
}

func Test_SessionFlags(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		// The flags which are given are the settings
		cmds := parseSession(t, "session-create", "--model", "ollama:llama3.2", "--system", "Be brief", "--tools", "memory:*", "--tools", "weather:*", "--opt", "temperature=0.5", "--schema", `{"type":"string"}`)
		meta := cmds.SessionCreate.SessionMeta
		assert.Equal(t, "ollama:llama3.2", types.Value(meta.Model))
		assert.Equal(t, "Be brief", types.Value(meta.System))
		assert.Equal(t, []string{"memory:*", "weather:*"}, meta.Tools)
		assert.JSONEq(t, `0.5`, string(meta.Opt["temperature"]))
		assert.JSONEq(t, `{"type":"string"}`, string(meta.Schema))
	})

	t.Run("UpdateNothing", func(t *testing.T) {
		// Flags which aren't given are left out, so they're kept
		id := uuid.New()
		cmds := parseSession(t, "session-update", id.String())
		meta := cmds.SessionUpdate.SessionMeta
		assert.Equal(t, id, cmds.SessionUpdate.ID)
		assert.Nil(t, meta.Model)
		assert.Nil(t, meta.System)
		assert.Nil(t, meta.Tools)
		assert.Nil(t, meta.Opt)
		assert.Nil(t, meta.Schema)

		data, err := json.Marshal(meta)
		require.NoError(t, err)
		assert.JSONEq(t, `{}`, string(data))
	})

	t.Run("UpdateClear", func(t *testing.T) {
		// An empty system prompt or tools clears them, null clears the
		// schema, and an option which is null is removed
		cmds := parseSession(t, "session-update", uuid.New().String(), "--system=", "--tools=", "--schema=null", "--opt", "thinking=null")
		meta := cmds.SessionUpdate.SessionMeta
		require.NotNil(t, meta.System)
		assert.Empty(t, *meta.System)
		assert.Nil(t, meta.Model)
		assert.NotNil(t, meta.Tools)
		assert.Empty(t, meta.Tools)
		assert.Equal(t, json.RawMessage(`null`), meta.Schema)

		data, err := json.Marshal(meta)
		require.NoError(t, err)
		assert.JSONEq(t, `{"system":"","tools":[],"opt":{"thinking":null},"schema":null}`, string(data))
	})
}

func Test_SessionID(t *testing.T) {
	// The identifier of a session is a UUID
	var cmds SessionCommands
	parser, err := kong.New(&cmds)
	require.NoError(t, err)
	_, err = parser.Parse([]string{"session", "not-a-uuid"})
	assert.Error(t, err)
}
