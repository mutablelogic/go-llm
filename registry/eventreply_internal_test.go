package registry

import (
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_eventReply(t *testing.T) {
	// The event agent's message, and importance, or, of another agent's
	// reply, its text, or its JSON, which is worth telling the person
	for reply, want := range map[string]struct {
		message    string
		importance schema.EventImportance
	}{
		`{"message":"Call your mother","importance":"alert"}`: {"Call your mother", schema.ImportanceAlert},
		`{"message":"Nothing new","importance":"ignore"}`:     {"Nothing new", schema.ImportanceIgnore},
		`{"message":"Hello"}`:                                 {"Hello", schema.ImportanceNotify},
		`{"message":""}`:                                      {"", schema.ImportanceNotify},
		`"Good morning"`:                                      {"Good morning", schema.ImportanceNotify},
		`{"text":"Good morning","language":"de"}`:             {`{"text":"Good morning","language":"de"}`, schema.ImportanceNotify},
		`[1,2]`: {`[1,2]`, schema.ImportanceNotify},
	} {
		message, importance, err := eventReply(json.RawMessage(reply))
		require.NoError(t, err, reply)
		assert.Equal(t, want.message, message, reply)
		assert.Equal(t, want.importance, importance, reply)
	}
	_, _, err := eventReply(json.RawMessage(`{"message":"x","importance":"urgent"}`))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
