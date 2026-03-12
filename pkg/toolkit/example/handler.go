package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	mcpclient "github.com/mutablelogic/go-llm/pkg/mcp/client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
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

func (h *handler) OnStateChange(c llm.Connector, state schema.ConnectorState) {
	slog.Info("connector state changed", "state", state)

	// Log out the current lists of tools, prompts, and resources on every state change for
	// visibility in this example. In a real implementation, you would likely want to be
	// more selective about when to log these.
	h.OnToolListChanged(c)
	h.OnPromptListChanged(c)
	h.OnResourceListChanged(c)
}

func (h *handler) OnToolListChanged(_ llm.Connector) {
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

func (h *handler) OnPromptListChanged(_ llm.Connector) {
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

func (h *handler) OnResourceListChanged(_ llm.Connector) {
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

func (h *handler) OnResourceUpdated(c llm.Connector, uri string) {
	slog.Info("resource updated", "uri", uri)
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
// onState is called once after the initial connection handshake to report the
// server's name (used by the toolkit to register the connector's namespace).
func (h *handler) CreateConnector(url string, onState func(schema.ConnectorState)) (llm.Connector, error) {
	w := &mcpConnector{onState: onState}

	c, err := mcpclient.New(url, "go-llm-example", "0.0.1",
		// Report the server name to the toolkit on first tool-list notification,
		// which fires once during Run's initial refresh after a successful connect.
		mcpclient.OptOnToolListChanged(func(ctx context.Context) {
			w.reportStateOnce()
		}),
		// Forward connector-level list-changed notifications to the handler.
		mcpclient.OptOnPromptListChanged(func(ctx context.Context) {
			slog.Info("prompt list changed", "url", url)
		}),
		mcpclient.OptOnResourceListChanged(func(ctx context.Context) {
			slog.Info("resource list changed", "url", url)
		}),
	)
	if err != nil {
		return nil, err
	}
	w.Client = c
	return w, nil
}

///////////////////////////////////////////////////////////////////////////////
// MCP CONNECTOR ADAPTER

// mcpConnector wraps *mcpclient.Client and calls onState once after the initial
// connection handshake using the server-reported name.
type mcpConnector struct {
	*mcpclient.Client
	onState func(schema.ConnectorState)
	once    sync.Once
}

// reportStateOnce is safe to call from any goroutine; it fires onState exactly
// once with the server information available from ServerInfo().
func (w *mcpConnector) reportStateOnce() {
	w.once.Do(func() {
		if w.onState == nil {
			return
		}
		name, version, _ := w.Client.ServerInfo()
		now := time.Now()
		state := schema.ConnectorState{
			ConnectedAt: &now,
		}
		if name != "" {
			state.Name = types.Ptr(name)
		}
		if version != "" {
			state.Version = types.Ptr(version)
		}
		w.onState(state)
	})
}
