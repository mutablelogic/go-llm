package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	mcpclient "github.com/mutablelogic/go-llm/pkg/mcp/client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type delegate struct {
	tk        toolkit.Toolkit
	mu        sync.Mutex
	connected int
	onReady   func()
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewDelegate() *delegate {
	return &delegate{}
}

func (d *delegate) SetToolkit(tk toolkit.Toolkit) {
	d.tk = tk
}

// SetOnReady registers a callback invoked (in a new goroutine) once both
// remote connectors have reported a successful connection.
func (d *delegate) SetOnReady(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onReady = fn
}

///////////////////////////////////////////////////////////////////////////////
// CALLBACKS

func (d *delegate) OnEvent(evt toolkit.ConnectorEvent) {
	switch evt.Kind {
	case toolkit.ConnectorEventStateChange:
		slog.Info("connector state changed", "state", evt.State, "connector", evt.Connector)
		if evt.State.ConnectedAt != nil {
			d.mu.Lock()
			d.connected++
			ready := d.connected == 2
			onReady := d.onReady
			d.mu.Unlock()
			if ready && onReady != nil {
				go onReady()
			}
		}
	case toolkit.ConnectorEventToolListChanged:
		d.logTools()
	case toolkit.ConnectorEventPromptListChanged:
		d.logPrompts()
	case toolkit.ConnectorEventResourceListChanged:
		d.logResources()
	case toolkit.ConnectorEventResourceUpdated:
		slog.Info("resource updated", "uri", evt.URI, "connector", evt.Connector)
	}
}

func (d *delegate) logTools() {
	if d.tk == nil {
		return
	}
	resp, err := d.tk.List(context.Background(), toolkit.ListRequest{Type: toolkit.ListTypeTools})
	if err != nil {
		slog.Error("failed to list tools", "error", err)
		return
	}
	for _, t := range resp.Tools {
		slog.Info("tool", "tool", fmt.Sprint(t))
	}
}

func (d *delegate) logPrompts() {
	if d.tk == nil {
		return
	}
	resp, err := d.tk.List(context.Background(), toolkit.ListRequest{Type: toolkit.ListTypePrompts})
	if err != nil {
		slog.Error("failed to list prompts", "error", err)
		return
	}
	for _, p := range resp.Prompts {
		slog.Info("prompt", "prompt", fmt.Sprint(p))
	}
}

func (d *delegate) logResources() {
	if d.tk == nil {
		return
	}
	resp, err := d.tk.List(context.Background(), toolkit.ListRequest{Type: toolkit.ListTypeResources})
	if err != nil {
		slog.Error("failed to list resources", "error", err)
		return
	}
	for _, r := range resp.Resources {
		slog.Info("resource", "resource", fmt.Sprint(r))
	}
}

///////////////////////////////////////////////////////////////////////////////
// METHODS

func (d *delegate) Call(ctx context.Context, p llm.Prompt, resources ...llm.Resource) (llm.Resource, error) {
	return nil, schema.ErrNotImplemented.With("prompt execution not supported in this example")
}

func (d *delegate) List(ctx context.Context, req toolkit.ListRequest) (*toolkit.ListResponse, error) {
	// Returns user-defined items
	return &toolkit.ListResponse{}, nil
}

// CreateConnector creates a new MCP HTTP connector for the given URL.
// onEvent is called to report lifecycle and list-change events back to the toolkit.
func (d *delegate) CreateConnector(url string, onEvent func(toolkit.ConnectorEvent)) (llm.Connector, error) {
	return mcpclient.New(url, "go-llm-example", "0.0.1",
		mcpclient.OptOnStateChange(func(ctx context.Context, state *schema.ConnectorState) {
			onEvent(toolkit.StateChangeEvent(types.Value(state)))
		}),
		mcpclient.OptOnToolListChanged(func(ctx context.Context) {
			onEvent(toolkit.ToolListChangeEvent())
		}),
		mcpclient.OptOnPromptListChanged(func(ctx context.Context) {
			onEvent(toolkit.PromptListChangeEvent())
		}),
		mcpclient.OptOnResourceListChanged(func(ctx context.Context) {
			onEvent(toolkit.ResourceListChangeEvent())
		}),
	)
}
