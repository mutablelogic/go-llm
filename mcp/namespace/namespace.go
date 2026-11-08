// Package namespace is the namespace of a connector's tools, in a toolkit:
// the tools of its MCP server, which are called with its client
package namespace

import (
	"context"
	"encoding/json"
	"slices"
	"sync"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	client "github.com/mutablelogic/go-llm/mcp/client"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Namespace is the namespace of a connector's tools, in a toolkit, which it
// has for as long as the connector exists: its MCP server's tools, as they
// were last listed, once it's connected, and none, while it isn't, which
// are listed again with Refresh, such as when the server says they changed
type Namespace struct {
	opt
	name string

	// Held as the tools are listed, or cleared, so they're in the order
	// they're asked for
	changing sync.Mutex

	mu          sync.RWMutex
	description string
	client      *client.Client    // The client, once it's connected, or nil
	match       func(string) bool // Whether a tool is included, by its name
	serial      uint64            // The serial of the connector, as it was connected
	tools       []*tool
	notify      func(eventschema.EventAction, string)
}

var _ toolkit.Namespace = (*Namespace)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace, with the name, such as one schema.Namespace
// returns, and the description, such as the connector's URL, until its
// server says its title, which has no tools, until it's connected, with the
// options, such as how long its tools have, with WithTimeout. Returns
// ErrBadParameter when the name is empty.
func New(name, description string, opts ...Opt) (*Namespace, error) {
	if name == "" {
		return nil, llm.ErrBadParameter.With("missing namespace")
	}
	ns := &Namespace{name: name, description: description}
	if err := ns.opt.apply(opts); err != nil {
		return nil, err
	}
	return ns, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - NAMESPACE

// Name returns the namespace's name
func (ns *Namespace) Name() string {
	return ns.name
}

// Type returns that the namespace's tools are an MCP server's
func (ns *Namespace) Type() string {
	return toolschema.NamespaceMCP
}

// Description returns what the namespace is, such as its server's title
func (ns *Namespace) Description() string {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.description
}

// SystemPrompt returns how to use the server, and its tools, as it said,
// when it was connected, or nothing, while it isn't
func (ns *Namespace) SystemPrompt() string {
	ns.mu.RLock()
	c := ns.client
	ns.mu.RUnlock()
	if c == nil {
		return ""
	}
	return c.Instructions()
}

// Tools returns the server's tools, as they were last listed, in order of
// their names, or none, while it isn't connected
func (ns *Namespace) Tools() []llm.Tool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	result := make([]llm.Tool, 0, len(ns.tools))
	for _, t := range ns.tools {
		result = append(result, t)
	}
	return result
}

// Notify sets the function which is called, once the namespace is added to
// a toolkit, when one of its tools is created, updated, or deleted, as
// they're listed again, or cleared
func (ns *Namespace) Notify(fn func(eventschema.EventAction, string)) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.notify = fn
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Connect lists the tools of the server the client is connected to, which
// are included, by the match, which includes every one when it's nil, and
// says each which was created, updated, or deleted, since they were last
// listed, with the description, such as the server's title, when it isn't
// empty, for the connector with the serial, unless it's been connected for
// a later one, such as once its patterns changed, when it does nothing.
// When they can't be listed, it has none, and that's the error.
func (ns *Namespace) Connect(ctx context.Context, c *client.Client, description string, match func(string) bool, serial uint64) error {
	if c == nil {
		return llm.ErrBadParameter.With("missing client")
	}
	if match == nil {
		match = func(string) bool { return true }
	}
	ns.changing.Lock()
	defer ns.changing.Unlock()
	ns.mu.Lock()
	if serial < ns.serial {
		// It's been connected for a later definition of its connector
		ns.mu.Unlock()
		return nil
	}
	ns.client, ns.match, ns.serial = c, match, serial
	if description != "" {
		ns.description = description
	}
	ns.mu.Unlock()
	if err := ns.list(ctx); err != nil {
		ns.clear()
		return err
	}
	return nil
}

// Refresh lists the tools again, of the server it's connected to, such as
// when it says they changed, or does nothing, while it isn't connected.
// When they can't be listed, they're as they were, and that's the error.
func (ns *Namespace) Refresh(ctx context.Context) error {
	ns.changing.Lock()
	defer ns.changing.Unlock()
	return ns.list(ctx)
}

// Disconnect clears the tools, as the client is disconnected from its
// server, and says each was deleted, so the namespace has none, until it's
// connected again, unless it's connected with another client, such as one
// which replaced it, when it does nothing
func (ns *Namespace) Disconnect(c *client.Client) {
	ns.changing.Lock()
	defer ns.changing.Unlock()
	ns.mu.RLock()
	other := ns.client != c
	ns.mu.RUnlock()
	if !other {
		ns.clear()
	}
}

// ConnectedTo returns true when the namespace has the tools of the server
// the client is connected to
func (ns *Namespace) ConnectedTo(c *client.Client) bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return c != nil && ns.client == c
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// list lists the tools of the server it's connected to, and replaces them,
// or does nothing, while it isn't connected. The caller holds changing.
func (ns *Namespace) list(ctx context.Context) error {
	ns.mu.RLock()
	c, match := ns.client, ns.match
	ns.mu.RUnlock()
	if c == nil {
		return nil
	}
	listed, err := c.ListTools(ctx)
	if err != nil {
		return err
	}
	tools := make([]*tool, 0, len(listed))
	for _, t := range listed {
		if t != nil && t.Name != "" && match(t.Name) {
			tools = append(tools, newTool(t, c, ns.timeout))
		}
	}
	slices.SortFunc(tools, func(a, b *tool) int { return compare(a.Name(), b.Name()) })
	ns.replace(tools)
	return nil
}

// clear forgets the client, and the tools. The caller holds changing.
func (ns *Namespace) clear() {
	ns.mu.Lock()
	ns.client = nil
	ns.mu.Unlock()
	ns.replace(nil)
}

// replace replaces the tools, and calls the function Notify set, without
// the lock, with each which was created, updated, or deleted
func (ns *Namespace) replace(tools []*tool) {
	ns.mu.Lock()
	changes := diff(ns.tools, tools)
	ns.tools = tools
	notify := ns.notify
	ns.mu.Unlock()
	if notify != nil {
		for _, change := range changes {
			notify(change.action, change.name)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// change is a tool which was created, updated, or deleted
type change struct {
	action eventschema.EventAction
	name   string
}

// diff returns the tools which were created, updated, or deleted, as the
// tools before became those after, in order of their names, where a tool
// which is updated is one whose definition changed
func diff(before, after []*tool) []change {
	old := make(map[string]*tool, len(before))
	for _, t := range before {
		old[t.Name()] = t
	}
	var result []change
	for _, t := range after {
		if prev, exists := old[t.Name()]; !exists {
			result = append(result, change{eventschema.EventCreate, t.Name()})
		} else if !sameTool(prev.t, t.t) {
			result = append(result, change{eventschema.EventUpdate, t.Name()})
		}
		delete(old, t.Name())
	}
	for _, t := range before {
		if _, exists := old[t.Name()]; exists {
			result = append(result, change{eventschema.EventDelete, t.Name()})
		}
	}
	return result
}

// sameTool returns true when the tools have the same definition, as JSON
func sameTool(a, b *sdkmcp.Tool) bool {
	x, err := json.Marshal(a)
	if err != nil {
		return false
	}
	y, err := json.Marshal(b)
	return err == nil && string(x) == string(y)
}

// compare returns how the strings are ordered
func compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
