package toolkit_test

import (
	"context"
	"sync"
	"testing"

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
// which tells the toolkit, unless it's told not to
type changingNamespace struct {
	mu     sync.Mutex
	tools  []llm.Tool
	notify func(eventschema.EventAction, string)
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

func (n *changingNamespace) Notify(fn func(eventschema.EventAction, string)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notify = fn
}

// set replaces the tools, and tells the toolkit, when told is true, that
// the tool with the name changed, as the action says
func (n *changingNamespace) set(action eventschema.EventAction, name string, told bool, tools ...llm.Tool) {
	n.mu.Lock()
	n.tools = tools
	notify := n.notify
	n.mu.Unlock()
	if told && notify != nil {
		notify(action, name)
	}
}

// toolEvent returns the event of the tool with the canonical name
func toolEvent(action eventschema.EventAction, canonical string) eventschema.Event {
	return eventschema.Event{Action: action, Schema: toolkit.EventSchema, Table: toolkit.ToolTable, Row: canonical}
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
	assert.Equal(t, []eventschema.Event{toolEvent(eventschema.EventCreate, "changing:second")}, drain(ch))
	tools, err := tk.Tools(ctx, "changing:second")
	require.NoError(t, err)
	assert.Equal(t, "changing__second", tools[0].Name())
	result, err := tk.CallTool(ctx, "changing:second", []byte(`{"a":1}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(result))

	// One which is updated is replaced, and one which is deleted can't be got
	ns.set(eventschema.EventUpdate, "second", true, echoTool{name: "first"}, echoTool{name: "second", err: llm.ErrForbidden})
	_, err = tk.CallTool(ctx, "changing:second", nil)
	assert.ErrorIs(t, err, llm.ErrForbidden)
	ns.set(eventschema.EventDelete, "second", true, echoTool{name: "first"})
	_, err = tk.Tools(ctx, "changing:second")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.Equal(t, []eventschema.Event{
		toolEvent(eventschema.EventUpdate, "changing:second"),
		toolEvent(eventschema.EventDelete, "changing:second"),
	}, drain(ch))

	// The toolkit follows the namespace as it is, so a change it's told of
	// out of order, such as a create after the tool was deleted, ends as
	// the namespace is
	ns.set(eventschema.EventCreate, "third", false, echoTool{name: "first"}, echoTool{name: "third"})
	ns.set(eventschema.EventDelete, "third", true, echoTool{name: "first"})
	ns.set(eventschema.EventCreate, "third", true, echoTool{name: "first"})
	_, err = tk.Tools(ctx, "changing:third")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.Empty(t, drain(ch))

	// A tool whose name isn't valid can't be called, so isn't added
	ns.set(eventschema.EventCreate, "bad__name", true, echoTool{name: "first"}, echoTool{name: "bad__name"})
	assert.Empty(t, drain(ch))
	list, err := tk.ListTools(ctx, schema.ToolListRequest{Namespace: "changing"})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), list.Count)
}

func TestNotifyWithoutBroadcaster(t *testing.T) {
	// A toolkit without a broadcaster follows the namespace's tools too
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	ns := &changingNamespace{}
	require.NoError(t, tk.Add(ns))
	ns.set(eventschema.EventCreate, "tool", true, echoTool{name: "tool"})
	_, err = tk.Tools(ctx, "changing:tool")
	assert.NoError(t, err)
}
