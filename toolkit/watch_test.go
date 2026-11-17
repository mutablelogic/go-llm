package toolkit_test

import (
	"context"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestWatchers(t *testing.T) {
	// Two watches, such as a toolkit's and an MCP server's, are each sent
	// the tools, once they're watching, and then each change, until they're
	// cancelled
	var w toolkit.Watchers
	tools := func() []llm.Tool { return []llm.Tool{echoTool{name: "first"}} }
	start := func() (chan eventschema.Event, context.CancelFunc, chan error) {
		ch := make(chan eventschema.Event)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- w.Watch(ctx, ch, tools) }()
		return ch, cancel, done
	}
	receive := func(ch chan eventschema.Event) eventschema.Event {
		t.Helper()
		select {
		case event := <-ch:
			return event
		case <-time.After(5 * time.Second):
			t.Fatal("no event")
			return eventschema.Event{}
		}
	}
	// both receives an event from each channel, in whichever order they're
	// sent
	both := func(a, b chan eventschema.Event) (eventschema.Event, eventschema.Event) {
		t.Helper()
		var x, y eventschema.Event
		for range 2 {
			select {
			case x = <-a:
			case y = <-b:
			case <-time.After(5 * time.Second):
				t.Fatal("no event")
			}
		}
		return x, y
	}
	a, cancelA, doneA := start()
	b, cancelB, doneB := start()
	x, y := both(a, b)
	assert.Equal(t, toolkit.ToolEvent(eventschema.EventCreate, "first"), x)
	assert.Equal(t, toolkit.ToolEvent(eventschema.EventCreate, "first"), y)

	// A change is waited for each watch to receive
	go w.Changed(eventschema.EventCreate, "second")
	x, y = both(a, b)
	assert.Equal(t, toolkit.ToolEvent(eventschema.EventCreate, "second"), x)
	assert.Equal(t, toolkit.ToolEvent(eventschema.EventCreate, "second"), y)

	// Once a watch is cancelled, it returns, and isn't waited for
	cancelA()
	require.NoError(t, <-doneA)
	go w.Changed(eventschema.EventDelete, "second")
	assert.Equal(t, toolkit.ToolEvent(eventschema.EventDelete, "second"), receive(b))
	cancelB()
	require.NoError(t, <-doneB)

	// A watch needs a channel
	assert.ErrorIs(t, w.Watch(context.Background(), nil, tools), llm.ErrBadParameter)
	assert.ErrorIs(t, toolkit.WatchTools(context.Background(), nil, nil), llm.ErrBadParameter)
}

func TestWatchTools(t *testing.T) {
	// A namespace whose tools don't change sends them, and waits
	ch := make(chan eventschema.Event, 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- toolkit.WatchTools(ctx, ch, []llm.Tool{echoTool{name: "a"}, echoTool{name: "b"}}) }()
	assert.Equal(t, toolkit.ToolEvent(eventschema.EventCreate, "a"), <-ch)
	assert.Equal(t, toolkit.ToolEvent(eventschema.EventCreate, "b"), <-ch)
	cancel()
	require.NoError(t, <-done)
}
