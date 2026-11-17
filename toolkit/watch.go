package toolkit

import (
	"context"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Watchers are the watches of a namespace whose tools change, which it
// tells, with Changed, so it can be watched by a toolkit, and an MCP server,
// at once. A namespace embeds it, or keeps one, and its Watch calls Watch.
// The zero value is ready to use, and it's safe to use concurrently.
type Watchers struct {
	mu      sync.Mutex
	watches map[*watch]struct{}
}

// watch is a channel which is sent the changes to a namespace's tools,
// until its context is cancelled
type watch struct {
	ctx context.Context
	ch  chan<- eventschema.Change
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// ToolEvent returns the event of the namespace's tool with the name, such
// as "current", rather than its canonical name, which was created, updated,
// or deleted, as the action says, which a namespace's Watch sends
func ToolEvent(action eventschema.ChangeAction, name string) eventschema.Change {
	return eventschema.Change{Action: action, Schema: EventSchema, Table: ToolTable, Row: name}
}

// WatchTools is the Watch of a namespace whose tools don't change: it sends
// that each of the tools was created, and blocks until the context is
// cancelled, and returns nil, or returns ErrBadParameter when there's no
// channel
func WatchTools(ctx context.Context, ch chan<- eventschema.Change, tools []llm.Tool) error {
	if ch == nil {
		return llm.ErrBadParameter.With("missing channel")
	}
	for _, tool := range tools {
		if tool != nil && !send(ctx, ch, ToolEvent(eventschema.ChangeCreate, tool.Name())) {
			return nil
		}
	}
	<-ctx.Done()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Watch sends the changes to the namespace's tools, which Changed is told,
// to the channel, until the context is cancelled, when it returns nil. Once
// it's watching, it sends that each of the tools, as tools returns them,
// was created, so a change between when the watch is started, and when it's
// watching, isn't missed. The changes, and the tools, which are sent at
// once, can be in any order, as each says which tool to read, as it is now,
// rather than how it changed. Each is waited for the channel to receive,
// so none is missed, and the channel is read until the context is
// cancelled. Returns ErrBadParameter when there's no channel.
func (w *Watchers) Watch(ctx context.Context, ch chan<- eventschema.Change, tools func() []llm.Tool) error {
	if ch == nil {
		return llm.ErrBadParameter.With("missing channel")
	}

	// It's watching, before the tools are read
	entry := &watch{ctx: ctx, ch: ch}
	w.mu.Lock()
	if w.watches == nil {
		w.watches = make(map[*watch]struct{})
	}
	w.watches[entry] = struct{}{}
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.watches, entry)
		w.mu.Unlock()
	}()

	// Send the tools, as they are, and wait until it's cancelled
	if tools != nil {
		for _, tool := range tools() {
			if tool != nil && !send(ctx, ch, ToolEvent(eventschema.ChangeCreate, tool.Name())) {
				return nil
			}
		}
	}
	<-ctx.Done()
	return nil
}

// Changed sends that the tool with the name was created, updated, or
// deleted, as the action says, to each watch, which it waits for, until it's
// received, or the watch is cancelled. It's called without the namespace's
// lock held, as a watch reads its tools.
func (w *Watchers) Changed(action eventschema.ChangeAction, name string) {
	w.mu.Lock()
	watches := make([]*watch, 0, len(w.watches))
	for entry := range w.watches {
		watches = append(watches, entry)
	}
	w.mu.Unlock()
	event := ToolEvent(action, name)
	for _, entry := range watches {
		send(entry.ctx, entry.ch, event)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// send sends the event to the channel, and returns true, or returns false,
// when the context is cancelled first
func send(ctx context.Context, ch chan<- eventschema.Change, event eventschema.Change) bool {
	select {
	case ch <- event:
		return true
	case <-ctx.Done():
		return false
	}
}
