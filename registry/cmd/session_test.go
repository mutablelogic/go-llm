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

// defaults are the command's defaults, in memory
type defaults map[string]string

func (d defaults) GetString(key string) string { return d[key] }

func Test_DefaultSession(t *testing.T) {
	id := uuid.New()

	t.Run("Optional", func(t *testing.T) {
		// The identifier can be left out of session and session-update, but
		// not session-delete
		cmds := parseSession(t, "session")
		assert.Equal(t, uuid.Nil(), cmds.SessionGet.ID)
		cmds = parseSession(t, "session-update", "--system", "Be brief")
		assert.Equal(t, uuid.Nil(), cmds.SessionUpdate.ID)
		assert.Equal(t, "Be brief", types.Value(cmds.SessionUpdate.System))

		var all SessionCommands
		parser, err := kong.New(&all)
		require.NoError(t, err)
		_, err = parser.Parse([]string{"session-delete"})
		assert.Error(t, err)
	})

	t.Run("Given", func(t *testing.T) {
		// An identifier which is given is used
		got, err := sessionID(defaults{defaultSessionKey: uuid.New().String()}, id)
		require.NoError(t, err)
		assert.Equal(t, id, got)
	})

	t.Run("Default", func(t *testing.T) {
		// Without an identifier, the default session is used
		got, err := sessionID(defaults{defaultSessionKey: id.String()}, uuid.Nil())
		require.NoError(t, err)
		assert.Equal(t, id, got)
	})

	t.Run("NoDefault", func(t *testing.T) {
		// Without an identifier, or a default session, there's no session
		_, err := sessionID(defaults{}, uuid.Nil())
		assert.ErrorContains(t, err, "missing session")
		_, err = sessionID(defaults{defaultSessionKey: "not-a-uuid"}, uuid.Nil())
		assert.Error(t, err)
	})
}

func Test_CreateMeta(t *testing.T) {
	// A session is created with the default model, unless one is given, and
	// an empty one has none
	d := defaults{defaultModelKey: "eliza:eliza-1966-en"}
	meta := createMeta(d, parseSession(t, "session-create").SessionCreate.SessionMeta)
	assert.Equal(t, "eliza:eliza-1966-en", types.Value(meta.Model))

	meta = createMeta(d, parseSession(t, "session-create", "--model", "ollama:llama3.2").SessionCreate.SessionMeta)
	assert.Equal(t, "ollama:llama3.2", types.Value(meta.Model))

	meta = createMeta(d, parseSession(t, "session-create", "--model=").SessionCreate.SessionMeta)
	require.NotNil(t, meta.Model)
	assert.Empty(t, *meta.Model)

	meta = createMeta(defaults{}, parseSession(t, "session-create").SessionCreate.SessionMeta)
	assert.Nil(t, meta.Model)
}

func Test_SessionOpts(t *testing.T) {
	// An option which isn't JSON is a JSON string, and null, numbers and
	// booleans are as they are
	meta := createMeta(defaults{}, parseSession(t, "session-create", "--opt", "effort=high", "--opt", "temperature=0.5", "--opt", "thinking=false", "--opt", "seed=null").SessionCreate.SessionMeta)
	data, err := json.Marshal(meta.Opt)
	require.NoError(t, err)
	assert.JSONEq(t, `{"effort":"high","temperature":0.5,"thinking":false,"seed":null}`, string(data))
}
