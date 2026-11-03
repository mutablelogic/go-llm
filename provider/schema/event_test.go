package schema_test

import (
	"encoding/json"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Event_JSON(t *testing.T) {
	t.Run("Event", func(t *testing.T) {
		// An event is its action, and what changed, which is left out when
		// it isn't set
		for _, test := range []struct {
			event schema.Event
			json  string
		}{
			{schema.Event{Action: schema.EventCreate, Schema: "llm", Table: "provider", Row: "ollama"}, `{"action":"create","schema":"llm","table":"provider","row":"ollama"}`},
			{schema.Event{Action: schema.EventUpdate, Table: "model"}, `{"action":"update","table":"model"}`},
			{schema.Event{Action: schema.EventDelete}, `{"action":"delete"}`},
		} {
			data, err := json.Marshal(test.event)
			require.NoError(t, err)
			assert.JSONEq(t, test.json, string(data))

			var out schema.Event
			require.NoError(t, json.Unmarshal(data, &out))
			assert.Equal(t, test.event, out)
		}
	})

	t.Run("Unknown", func(t *testing.T) {
		// An action must be one of the actions
		var out schema.Event
		assert.Error(t, json.Unmarshal([]byte(`{"action":"INSERT","table":"model"}`), &out))
		var action schema.EventAction
		assert.Error(t, json.Unmarshal([]byte(`"truncate"`), &action))
		assert.Error(t, json.Unmarshal([]byte(`1`), &action))
	})
}
