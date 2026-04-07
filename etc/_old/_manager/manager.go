package manager

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	store "github.com/mutablelogic/go-llm/pkg/_store"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	types "github.com/mutablelogic/go-server/pkg/types"
	trace "go.opentelemetry.io/otel/trace"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Manager struct {
	ctx              context.Context
	cancel           context.CancelFunc
	clients          map[string]llm.Client
	sessionStore     schema.SessionStore
	agentStore       schema.AgentStore
	connectorStore   schema.ConnectorStore
	toolkit          *tool.Toolkit
	toolkitOpts      []tool.ToolkitOpt
	connectorFactory ConnectorFactory
	tracer           trace.Tracer
	serverName       string
	serverVersion    string
	logger           *slog.Logger
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewManager(name, ver string, opts ...Opt) (*Manager, error) {
	// Create the manager
	m := new(Manager)
	m.ctx, m.cancel = context.WithCancel(context.Background())

	// Validate required parameters
	if name = strings.TrimSpace(name); name == "" {
		return nil, schema.ErrBadParameter.With("server name is required")
	} else {
		m.serverName = name
		m.serverVersion = strings.TrimSpace(ver)
		m.clients = make(map[string]llm.Client)
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, err
		}
	}

	// Default logger to slog.Default() if not set by an option.
	if m.logger == nil {
		m.logger = slog.Default()
	}

	// Default to in-memory session store if none was provided
	if m.sessionStore == nil {
		m.sessionStore = store.NewMemorySessionStore()
	}

	// Default to in-memory agent store if none was provided
	if m.agentStore == nil {
		m.agentStore = store.NewMemoryAgentStore()
	}

	// Default to in-memory connector store if none was provided
	if m.connectorStore == nil {
		m.connectorStore = store.NewMemoryConnectorStore()
	}

	// By default, we don't configure a credential store
	// since it requires a passphrase.

	// Build the toolkit with the accumulated opts and a state writeback hook.
	tk, err := tool.NewToolkit(append(m.toolkitOpts,
		tool.WithLogHandler(m.onConnectorLog),
		tool.WithStateHandler(m.onConnectorState),
		tool.WithToolsHandler(m.onConnectorTools),
	)...)
	if err != nil {
		return nil, err
	}
	m.toolkit = tk

	// Replay persisted enabled connectors into the toolkit.
	// Per-connector failures are logged but do not abort startup.
	if m.connectorFactory != nil {
		m.replayConnectors()
	}

	// Return success
	return m, nil
}

func (m *Manager) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.toolkit != nil {
		return m.toolkit.Close()
	}
	return nil
}

// replayConnectors loads all enabled connectors from the store and wires them
// into the toolkit. Individual failures are logged and skipped.
func (m *Manager) replayConnectors() {
	ctx := context.Background()
	enabled := true
	resp, err := m.connectorStore.ListConnectors(ctx, schema.ConnectorListRequest{Enabled: &enabled})
	if err != nil {
		m.logger.Warn("failed to list connectors on startup", "err", err)
		return
	}
	for _, c := range resp.Body {
		conn, err := m.connectorFactory(ctx, c.URL)
		if err != nil {
			m.logger.Warn("connector factory failed on startup", "url", c.URL, "err", err)
			continue
		}
		if err := m.toolkit.AddConnector(c.URL, conn); err != nil {
			m.logger.Warn("failed to add connector to toolkit on startup", "url", c.URL, "err", err)
			continue
		}
		m.logger.Debug("connector queued for connection", "url", c.URL, "namespace", types.Value(c.Namespace))
	}
}

// onConnectorLog forwards log messages from a connector's MCP session to the manager logger.
// error values in args are converted to their string representation so they display
// correctly with all slog handlers (some handlers JSON-marshal values directly,
// turning an unexported error struct into "{}").
func (m *Manager) onConnectorLog(url string, level slog.Level, msg string, args ...any) {
	for i := 1; i < len(args); i += 2 {
		if err, ok := args[i].(error); ok {
			args[i] = err.Error()
		}
	}
	m.logger.Log(context.Background(), level, msg, append(args, "url", url)...)
}

// onConnectorState writes connector state back to the connector store.
// A zero ConnectedAt signals that the connector has disconnected.
func (m *Manager) onConnectorState(url string, state schema.ConnectorState) {
	_, _ = m.connectorStore.UpdateConnectorState(context.Background(), url, state)
	if state.ConnectedAt == nil || state.ConnectedAt.IsZero() {
		m.logger.Info("connector disconnected", "url", url)
	} else if name := types.Value(state.Name); name == "" {
		m.logger.Info("connector connected", "url", url)
	} else {
		m.logger.Info("connector connected", "url", url, "name", name, "version", types.Value(state.Version))
	}
}

// onConnectorTools logs when a connector's tool list changes.
func (m *Manager) onConnectorTools(url string, tools []llm.Tool) {
	if tools == nil {
		m.logger.Debug("connector tools cleared", "url", url)
	} else {
		m.logger.Info("connector tools updated", "url", url, "count", len(tools))
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *Manager) clientForModel(model *schema.Model) llm.Client {
	if model == nil {
		return nil
	}
	return m.clients[model.OwnedBy]
}

// convertOptsForClient applies options once, resolves any deferred client-aware
// options, then re-applies the combined set to produce a flat option slice.
func convertOptsForClient(opts []opt.Opt, client llm.Client) ([]opt.Opt, error) {
	// First pass: apply options to collect any WithClient markers
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Resolve client-aware options by provider name
	resolved, err := opt.ConvertOptsForClient(o, client.Name())
	if err != nil {
		return nil, err
	}

	// Return original opts plus the resolved provider-specific opts
	return append(opts, resolved...), nil
}

func (m *Manager) getModel(ctx context.Context, provider, model string) (*schema.Model, error) {
	if provider := strings.TrimSpace(provider); provider == "" {
		// Search all clients for the model in parallel
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		var mu sync.Mutex
		var result *schema.Model

		g, ctx := errgroup.WithContext(ctx)
		for _, client := range m.clients {
			g.Go(func() error {
				models, err := client.ListModels(ctx)
				if err != nil {
					return nil // Swallow per-provider errors
				}

				mu.Lock()
				defer mu.Unlock()
				if result != nil {
					return nil // Already found
				}
				for _, m := range models {
					if m.Name == model {
						result = &m
						cancel()
						return nil
					}
				}
				return nil
			})
		}
		g.Wait()

		if result != nil {
			return result, nil
		}
		return nil, schema.ErrNotFound.Withf("model %q not found in any provider", model)
	} else if client, ok := m.clients[provider]; !ok {
		return nil, schema.ErrNotFound.Withf("no client found for provider %q", provider)
	} else {
		return client.GetModel(ctx, model)
	}
}
