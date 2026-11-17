package toolkit_test

import (
	"context"
	"sync"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// changingNamespace is a namespace whose tools change once it's added,
// which tells its watches, unless it's told not to
type changingNamespace struct {
	mu       sync.Mutex
	tools    []llm.Tool
	watchers toolkit.Watchers
}

func (n *changingNamespace) Name() string         { return "changing" }
func (n *changingNamespace) SystemPrompt() string { return "" }

func (n *changingNamespace) Tools() []llm.Tool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]llm.Tool(nil), n.tools...)
}

func (*changingNamespace) Description() string { return "" }

// Type returns that the namespace's tools are built in
func (*changingNamespace) Type() string { return schema.NamespaceBuiltin }

func (n *changingNamespace) Watch(ctx context.Context, ch chan<- eventschema.Event) error {
	return n.watchers.Watch(ctx, ch, n.Tools)
}

// set replaces the tools, and tells the watches, when told is true, that
// the tool with the name changed, as the action says
func (n *changingNamespace) set(action eventschema.EventAction, name string, told bool, tools ...llm.Tool) {
	n.mu.Lock()
	n.tools = tools
	n.mu.Unlock()
	if told {
		n.watchers.Changed(action, name)
	}
}

// toolEvent returns the event of the tool with the canonical name
func toolEvent(action eventschema.EventAction, canonical string) eventschema.Event {
	return eventschema.Event{Action: action, Schema: toolkit.EventSchema, Table: toolkit.ToolTable, Row: canonical}
}

// next returns the next n events which are published to the channel, as
// the toolkit applies the changes it's told of, which it does as it reads
// them, or fails, when they aren't published in time
func next(t *testing.T, ch chan eventschema.Event, n int) []eventschema.Event {
	t.Helper()
	var result []eventschema.Event
	for len(result) < n {
		select {
		case event := <-ch:
			result = append(result, event)
		case <-time.After(5 * time.Second):
			t.Fatalf("got %d of %d events: %v", len(result), n, result)
		}
	}
	return result
}

// drain returns the events which have been published to the channel
func drain(ch chan eventschema.Event) []eventschema.Event {
	var result []eventschema.Event
	for {
		select {
		case event := <-ch:
			result = append(result, event)
		default:
			return result
		}
	}
}

func TestNotify(t *testing.T) {
	ctx := context.Background()
	b := broadcaster.New()
	ch := make(chan eventschema.Event, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, err := b.Watch(watchCtx, ch)
	require.NoError(t, err)
	tk, err := toolkit.New(ctx, toolkit.WithBroadcaster(b))
	require.NoError(t, err)

	// The namespace's tools, when it's added, are created
	ns := &changingNamespace{tools: []llm.Tool{echoTool{name: "first"}}}
	require.NoError(t, tk.Add(ns))
	assert.Equal(t, []eventschema.Event{toolEvent(eventschema.EventCreate, "changing:first")}, drain(ch))

	// A tool which is created can be got and called, by its canonical name
	ns.set(eventschema.EventCreate, "second", true, echoTool{name: "first"}, echoTool{name: "second"})
	assert.Equal(t, []eventschema.Event{toolEvent(eventschema.EventCreate, "changing:second")}, next(t, ch, 1))
	tools, err := tk.Tools(ctx, "changing:second")
	require.NoError(t, err)
	assert.Equal(t, "changing__second", tools[0].Name())
	result, err := tk.CallTool(ctx, "changing:second", []byte(`{"a":1}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(result))

	// One which is updated is replaced, and one which is deleted can't be got
	ns.set(eventschema.EventUpdate, "second", true, echoTool{name: "first"}, echoTool{name: "second", err: llm.ErrForbidden})
	assert.Equal(t, []eventschema.Event{toolEvent(eventschema.EventUpdate, "changing:second")}, next(t, ch, 1))
	_, err = tk.CallTool(ctx, "changing:second", nil)
	assert.ErrorIs(t, err, llm.ErrForbidden)
	ns.set(eventschema.EventDelete, "second", true, echoTool{name: "first"})
	assert.Equal(t, []eventschema.Event{toolEvent(eventschema.EventDelete, "changing:second")}, next(t, ch, 1))
	_, err = tk.Tools(ctx, "changing:second")
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// The toolkit follows the namespace as it is, so a change it's told of
	// out of order, such as a create after the tool was deleted, ends as
	// the namespace is
	ns.set(eventschema.EventCreate, "third", false, echoTool{name: "first"}, echoTool{name: "third"})
	ns.set(eventschema.EventDelete, "third", true, echoTool{name: "first"})
	ns.set(eventschema.EventCreate, "third", true, echoTool{name: "first"})
	_, err = tk.Tools(ctx, "changing:third")
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// A tool whose name isn't valid can't be called, so isn't added. The
	// changes are applied in order, so once the tool after it is published,
	// nothing else was.
	ns.set(eventschema.EventCreate, "bad__name", true, echoTool{name: "first"}, echoTool{name: "bad__name"})
	ns.set(eventschema.EventCreate, "fourth", true, echoTool{name: "first"}, echoTool{name: "fourth"})
	assert.Equal(t, []eventschema.Event{toolEvent(eventschema.EventCreate, "changing:fourth")}, next(t, ch, 1))
	assert.Empty(t, drain(ch))
	list, err := tk.ListTools(ctx, schema.ToolListRequest{Namespace: "changing"})
	require.NoError(t, err)
	assert.Equal(t, uint64(2), list.Count)
}

func TestNotifyWithoutBroadcaster(t *testing.T) {
	// A toolkit without a broadcaster follows the namespace's tools too
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	ns := &changingNamespace{}
	require.NoError(t, tk.Add(ns))
	ns.set(eventschema.EventCreate, "tool", true, echoTool{name: "tool"})
	assert.Eventually(t, func() bool {
		_, err := tk.Tools(ctx, "changing:tool")
		return err == nil
	}, 5*time.Second, 5*time.Millisecond)
}

// sliceTool is a tool which can't be compared, as it has a slice
type sliceTool struct {
	echoTool
	tags []string
}

func TestUncomparable(t *testing.T) {
	// A tool which can't be compared is as the namespace returns it now,
	// without being published as updated, until the namespace says it was
	ctx := context.Background()
	b := broadcaster.New()
	ch := make(chan eventschema.Event, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, err := b.Watch(watchCtx, ch)
	require.NoError(t, err)
	tk, err := toolkit.New(ctx, toolkit.WithBroadcaster(b))
	require.NoError(t, err)
	ns := &changingNamespace{tools: []llm.Tool{sliceTool{echoTool: echoTool{name: "tool"}}}}
	require.NoError(t, tk.Add(ns))
	assert.Equal(t, []eventschema.Event{toolEvent(eventschema.EventCreate, "changing:tool")}, next(t, ch, 1))

	// It's replaced, without being said, and is called as it is now
	ns.set(eventschema.EventUpdate, "tool", false, sliceTool{echoTool: echoTool{name: "tool", err: llm.ErrForbidden}})
	_, err = tk.CallTool(ctx, "changing:tool", nil)
	assert.ErrorIs(t, err, llm.ErrForbidden)
	assert.Empty(t, drain(ch))

	// And is published, once it's said, which is said until the toolkit is
	// watching, as it starts to once it's added
	require.Eventually(t, func() bool {
		ns.set(eventschema.EventUpdate, "tool", true, sliceTool{echoTool: echoTool{name: "tool", err: llm.ErrForbidden}})
		select {
		case event := <-ch:
			return assert.Equal(t, toolEvent(eventschema.EventUpdate, "changing:tool"), event)
		case <-time.After(10 * time.Millisecond):
			return false
		}
	}, 5*time.Second, time.Millisecond)
}

func TestClose(t *testing.T) {
	// Once it's closed, a toolkit doesn't watch its namespaces, but still
	// reads their tools, as it's asked for them
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	ns := &changingNamespace{}
	require.NoError(t, tk.Add(ns))
	require.NoError(t, tk.Close())
	ns.set(eventschema.EventCreate, "tool", true, echoTool{name: "tool"})
	_, err = tk.Tools(ctx, "changing:tool")
	assert.NoError(t, err)
}
