package heartbeat_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Tool(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	c := &clock{}
	c.Set("2026-01-05T08:00:00Z")
	h, _, _ := newHeartbeats(t, heartbeat.NewStore(), c, heartbeat.WithZone(func() *time.Location { return berlin }))
	tools := h.Tools()
	require.Len(t, tools, 1)
	tool := tools[0]
	assert.Contains(t, h.SystemPrompt(), "Monday, 5 January 2026, 09:00 CET")
	assert.Contains(t, h.SystemPrompt(), "the person's, Europe/Berlin")

	// run calls the tool, in the session, and returns its result, as JSON
	session := uuid.New()
	run := func(id uuid.UUID, input string) (map[string]any, error) {
		ctx := toolkit.WithSession(t.Context(), toolkit.NewSession("heartbeats", id, func(llm.StreamType, string) {}, nil))
		result, err := tool.Run(ctx, json.RawMessage(input))
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, json.Unmarshal(data, &got))
		return got, nil
	}

	// Scheduled, at a time without an offset, which is the person's
	got, err := run(session, `{"operation":"schedule","title":"Call","prompt":"Remind David to call his mother","at":"2026-01-05T18:00"}`)
	require.NoError(t, err)
	assert.Equal(t, "2026-01-05T18:00:00+01:00", got["next"])
	id := got["id"].(string)

	// Not without a session
	_, err = run(uuid.Nil(), `{"operation":"schedule","title":"Call","prompt":"x","at":"2026-01-05T18:00"}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)

	// Listed, for the session, and updated to a schedule, and cancelled
	got, err = run(session, `{"operation":"list"}`)
	require.NoError(t, err)
	assert.Equal(t, float64(1), got["count"])
	got, err = run(uuid.New(), `{"operation":"list"}`)
	require.NoError(t, err)
	assert.Equal(t, float64(0), got["count"])
	got, err = run(session, `{"operation":"update","id":"`+id+`","schedule":"0 18 * * *"}`)
	require.NoError(t, err)
	assert.Equal(t, "0 18 * * *", got["schedule"])
	assert.Nil(t, got["at"])
	_, err = run(session, `{"operation":"cancel","id":"`+id+`"}`)
	require.NoError(t, err)
	list, err := h.List(t.Context(), schema.HeartbeatListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(0), list.Count)

	// Unknown operations, and identifiers
	_, err = run(session, `{"operation":"snooze"}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = run(session, `{"operation":"cancel","id":"x"}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, err = run(session, `{"operation":"cancel","id":"`+uuid.New().String()+`"}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.False(t, strings.Contains(h.SystemPrompt(), "%!"))
}
