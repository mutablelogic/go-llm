package main

import (
	"context"
	"fmt"
	"log/slog"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	mcpclient "github.com/mutablelogic/go-llm/pkg/mcp/client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type handler struct {
	tk toolkit.Toolkit
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewHandler() *handler {
	return &handler{}
}

func (h *handler) SetToolkit(tk toolkit.Toolkit) {
	h.tk = tk
}

///////////////////////////////////////////////////////////////////////////////
// CALLBACKS

func (h *handler) OnEvent(evt toolkit.ConnectorEvent) {
	switch evt.Kind {
	case toolkit.ConnectorEventStateChange:
		slog.Info("connector state changed", "state", evt.State)
		// Log out the current lists of tools, prompts, and resources on every
		// state change for visibility. In a real implementation you would likely
		// be more selective.
		h.logTools()
		h.logPrompts()
		h.logResources()
	case toolkit.ConnectorEventToolListChanged:
		h.logTools()
	case toolkit.ConnectorEventPromptListChanged:
		h.logPrompts()
	case toolkit.ConnectorEventResourceListChanged:
		h.logResources()
	case toolkit.ConnectorEventResourceUpdated:
		slog.Info("resource updated", "uri", evt.URI)
	}
}

func (h *handler) logTools() {
	if h.tk == nil {
		return
	}
	resp, err := h.tk.List(context.Background(), toolkit.ListRequest{Type: toolkit.ListTypeTools})
	if err != nil {
		slog.Error("failed to list tools", "error", err)
		return
	}
	for _, t := range resp.Tools {
		slog.Info("tool", "tool", fmt.Sprint(t))
	}
}

func (h *handler) logPrompts() {
	if h.tk == nil {
		return
	}
	resp, err := h.tk.List(context.Background(), toolkit.ListRequest{Type: toolkit.ListTypePrompts})
	if err != nil {
		slog.Error("failed to list prompts", "error", err)
		return
	}
	for _, p := range resp.Prompts {
		slog.Info("prompt", "prompt", fmt.Sprint(p))
	}
}

func (h *handler) logResources() {
	if h.tk == nil {
		return
	}
	resp, err := h.tk.List(context.Background(), toolkit.ListRequest{Type: toolkit.ListTypeResources})
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

func (h *handler) Call(ctx context.Context, p llm.Prompt, resources ...llm.Resource) (llm.Resource, error) {
	return nil, llm.ErrNotImplemented.With("prompt execution not supported in this example")
}

func (h *handler) List(ctx context.Context, req toolkit.ListRequest) (*toolkit.ListResponse, error) {
	// Returns user-defined items
	return &toolkit.ListResponse{}, nil
}

// CreateConnector creates a new MCP HTTP connector for the given URL.
// onEvent is called to report lifecycle and list-change events back to the toolkit.
func (h *handler) CreateConnector(url string, onEvent func(toolkit.ConnectorEvent)) (llm.Connector, error) {
	return mcpclient.New(url, "go-llm-example", "0.0.1",
		mcpclient.OptOnStateChange(func(ctx context.Context, state *schema.ConnectorState) {
			onEvent(toolkit.StateChangeEvent(*state))
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
