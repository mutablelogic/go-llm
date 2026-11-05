package memory_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	memory "github.com/mutablelogic/go-llm/memory"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Memories(t *testing.T) {
	ctx := t.Context()
	b := broadcaster.New()
	ch := make(chan eventschema.Event, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	done, err := b.Watch(watchCtx, ch)
	require.NoError(t, err)
	defer func() {
		cancel()
		assert.NoError(t, <-done)
	}()
	m, err := memory.New(ctx, memory.NewStore(), memory.WithBroadcaster(b))
	require.NoError(t, err)
	clock := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	memory.SetClock(m, func() time.Time { return clock })

	// Remembered, as it's created
	lights := schema.MemoryMeta{Key: "kitchen_lights", Description: "The kitchen's lights", Body: "light.kitchen_main and light.kitchen_counter"}
	got, created, err := m.Remember(ctx, lights, 0)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, uint64(1), got.Version)
	assert.Equal(t, clock, got.Created)

	// The same again, at its version, changes nothing, and a change, at its
	// version, replaces it
	clock = clock.Add(time.Hour)
	got, created, err = m.Remember(ctx, schema.MemoryMeta{Key: " kitchen_lights ", Description: "The kitchen's lights ", Body: lights.Body + "\n"}, 1)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, uint64(1), got.Version)
	lights.Body = "light.kitchen_main"
	got, created, err = m.Remember(ctx, lights, 1)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, uint64(2), got.Version)
	assert.Equal(t, clock, got.Modified)

	// Not at its version, which is a conflict, nor created, when it exists,
	// nor replaced, when it doesn't
	for _, version := range []uint64{0, 1, 3} {
		_, _, err = m.Remember(ctx, lights, version)
		assert.ErrorIs(t, err, llm.ErrConflict, version)
	}
	_, _, err = m.Remember(ctx, schema.MemoryMeta{Key: "units", Description: "Units", Body: "Metric"}, 1)
	assert.ErrorIs(t, err, llm.ErrConflict)

	clock = clock.Add(time.Hour)
	_, _, err = m.Remember(ctx, schema.MemoryMeta{Key: "units", Description: "Units the person prefers", Body: "Metric, and Celsius"}, 0)
	require.NoError(t, err)

	// Invalid memories
	for _, meta := range []schema.MemoryMeta{
		{Key: "not a key", Description: "x", Body: "x"},
		{Key: "x", Body: "x"},
		{Key: "x", Description: "two\nlines", Body: "x"},
		{Key: "x", Description: "x"},
	} {
		_, _, err := m.Remember(ctx, meta, 0)
		assert.ErrorIs(t, err, llm.ErrBadParameter, meta.Key)
	}

	// Listed, the most recently changed first, and searched, by each word,
	// in any case
	list, err := m.ListMemories(ctx, schema.MemoryListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	require.Len(t, list.Body, 2)
	assert.Equal(t, "units", list.Body[0].Key)
	list, err = m.ListMemories(ctx, schema.MemoryListRequest{Q: "KITCHEN main"})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "kitchen_lights", list.Body[0].Key)
	list, err = m.ListMemories(ctx, schema.MemoryListRequest{Q: "kitchen celsius"})
	require.NoError(t, err)
	assert.Empty(t, list.Body)
	list, err = m.ListMemories(ctx, schema.MemoryListRequest{Offset: 1, Limit: types.Ptr(uint64(5))})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "kitchen_lights", list.Body[0].Key)

	// Forgotten
	forgotten, err := m.Forget(ctx, "units")
	require.NoError(t, err)
	assert.Equal(t, "Metric, and Celsius", forgotten.Body)
	_, err = m.Forget(ctx, "units")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = m.GetMemory(ctx, "units")
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// The changes, as events
	var events []eventschema.Event
	for len(ch) > 0 {
		events = append(events, <-ch)
	}
	event := func(action eventschema.EventAction, key, data string) eventschema.Event {
		event := eventschema.Event{Action: action, Schema: "memory", Table: memory.MemoryTable, Row: key}
		if data != "" {
			event.Data = json.RawMessage(data)
		}
		return event
	}
	assert.Equal(t, []eventschema.Event{
		event(eventschema.EventCreate, "kitchen_lights", `{"version":1}`),
		event(eventschema.EventUpdate, "kitchen_lights", `{"version":2}`),
		event(eventschema.EventCreate, "units", `{"version":1}`),
		event(eventschema.EventDelete, "units", ""),
	}, events)
}

func Test_ForgetLine(t *testing.T) {
	ctx := t.Context()
	m := newMemory(t, time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC))
	_, _, err := m.Remember(ctx, schema.MemoryMeta{Key: "family", Description: "The person's family", Body: "- Sister Cath, in Leeds\n- Brother Tom\n- Brother Tom"}, 0)
	require.NoError(t, err)

	// A line, with or without its bullet, or the space around it, which is
	// taken out wherever it is, and changes its version
	got, err := m.ForgetLine(ctx, "family", "  Brother Tom ")
	require.NoError(t, err)
	assert.Equal(t, "- Sister Cath, in Leeds", got.Body)
	assert.Equal(t, uint64(2), got.Version)
	_, err = m.ForgetLine(ctx, "family", "Brother Tom")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	_, err = m.ForgetLine(ctx, "nobody", "x")
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// The last line forgets the memory
	_, err = m.ForgetLine(ctx, "family", "- Sister Cath, in Leeds")
	require.NoError(t, err)
	_, err = m.GetMemory(ctx, "family")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_MemoryTools(t *testing.T) {
	m := newMemory(t, time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC))
	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	require.NoError(t, tk.Add(m))
	call := func(input string) (map[string]any, error) {
		result, err := tk.CallTool(t.Context(), "memory:memories", json.RawMessage(input))
		if err != nil {
			return nil, err
		}
		var output map[string]any
		require.NoError(t, json.Unmarshal(result, &output))
		return output, nil
	}

	output, err := call(`{"operation":"remember","key":"units","description":"Units the person prefers","body":"- Metric\n- Celsius"}`)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key": "units", "version": float64(1), "created": true}, output)

	// Recalled, with its version, which a change needs
	output, err = call(`{"operation":"recall","key":"units"}`)
	require.NoError(t, err)
	assert.Equal(t, "- Metric\n- Celsius", output["body"])
	assert.Equal(t, float64(1), output["version"])
	_, err = call(`{"operation":"remember","key":"units","description":"Units the person prefers","body":"- Imperial"}`)
	assert.ErrorContains(t, err, "is at version 1")
	output, err = call(`{"operation":"remember","key":"units","description":"Units the person prefers","body":"- Metric\n- Celsius\n- 24-hour clock","version":1}`)
	require.NoError(t, err)
	assert.Equal(t, float64(2), output["version"])

	output, err = call(`{"operation":"search","q":"metric"}`)
	require.NoError(t, err)
	assert.Equal(t, float64(1), output["count"])
	output, err = call(`{"operation":"search"}`)
	require.NoError(t, err)
	assert.Equal(t, float64(1), output["count"])
	output, err = call(`{"operation":"search","q":"imperial"}`)
	require.NoError(t, err)
	assert.Equal(t, float64(0), output["count"])
	assert.Equal(t, []any{}, output["memories"])

	// A line is forgotten, and then the whole memory
	output, err = call(`{"operation":"forget","key":"units","line":"Celsius"}`)
	require.NoError(t, err)
	assert.Equal(t, "- Metric\n- 24-hour clock", output["body"])
	output, err = call(`{"operation":"forget","key":"units"}`)
	require.NoError(t, err)
	assert.Equal(t, float64(3), output["version"])

	// Errors: a memory which isn't there, one without what it needs, and an
	// operation which isn't one
	for _, input := range []string{
		`{"operation":"recall","key":"units"}`,
		`{"operation":"forget","key":"units"}`,
		`{"operation":"remember","key":"units"}`,
		`{"operation":"recall"}`,
		`{"operation":"list"}`,
	} {
		_, err := call(input)
		assert.Error(t, err, input)
	}
}
