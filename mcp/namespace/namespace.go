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
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Namespace is the namespace of a connector's tools, which are its MCP
// server's, as they were last listed, which are listed again with Refresh,
// such as when the server says they changed
type Namespace struct {
	name        string
	description string
	client      *client.Client

	mu     sync.RWMutex
	tools  []*tool
	notify func(eventschema.EventAction, string)
}

var _ toolkit.Namespace = (*Namespace)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace, with the name, such as one schema.Namespace
// returns, and the description, such as the server's title, of the tools
// of the server the client is connected to, which are listed. Returns
// ErrBadParameter when the name is empty, or there's no client, and the
// client's error, when its tools can't be listed, such as
// ErrServiceUnavailable, when it isn't connected.
func New(ctx context.Context, name, description string, c *client.Client) (*Namespace, error) {
	if name == "" {
		return nil, llm.ErrBadParameter.With("missing namespace")
	} else if c == nil {
		return nil, llm.ErrBadParameter.With("missing client")
	}
	ns := &Namespace{name: name, description: description, client: c}
	if err := ns.Refresh(ctx); err != nil {
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

// Description returns what the namespace is, such as its server's title
func (ns *Namespace) Description() string {
	return ns.description
}

// SystemPrompt returns how to use the server, and its tools, as it said,
// when it was connected, or nothing
func (ns *Namespace) SystemPrompt() string {
	return ns.client.Instructions()
}

// Tools returns the server's tools, as they were last listed, in order of
// their names
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
// they're listed again
func (ns *Namespace) Notify(fn func(eventschema.EventAction, string)) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.notify = fn
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Refresh lists the server's tools again, and calls the function Notify
// set, without the lock, with each which was created, updated, or deleted,
// since they were last listed. When they can't be listed, they're as they
// were, and that's the error.
func (ns *Namespace) Refresh(ctx context.Context) error {
	listed, err := ns.client.ListTools(ctx)
	if err != nil {
		return err
	}
	tools := make([]*tool, 0, len(listed))
	for _, t := range listed {
		if t != nil && t.Name != "" {
			tools = append(tools, newTool(t, ns.client))
		}
	}
	slices.SortFunc(tools, func(a, b *tool) int { return compare(a.Name(), b.Name()) })

	// Replace them, and say what changed
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
	return nil
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
