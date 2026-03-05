package manager

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	types "github.com/mutablelogic/go-server/pkg/types"
	trace "go.opentelemetry.io/otel/trace"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Manager struct {
	clients          map[string]llm.Client
	sessionStore     schema.SessionStore
	agentStore       schema.AgentStore
	credentialStore  schema.CredentialStore
	connectorStore   schema.ConnectorStore
	toolkit          *tool.Toolkit
	toolkitOpts      []tool.ToolkitOpt
	connectorFactory ConnectorFactory
	tracer           trace.Tracer
	serverName       string
	serverVersion    string
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewManager(name, ver string, opts ...Opt) (*Manager, error) {
	// Create the manager
	m := new(Manager)

	// Validate required parameters
	if name = strings.TrimSpace(name); name == "" {
		return nil, llm.ErrBadParameter.With("server name is required")
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
	resp, err := m.connectorStore.ListConnectors(ctx, schema.ListConnectorsRequest{Enabled: &enabled})
	if err != nil {
		slog.Warn("failed to list connectors on startup", "err", err)
		return
	}
	for _, c := range resp.Body {
		conn, err := m.connectorFactory(ctx, c.URL, m.credOptsFor(ctx, c.URL)...)
		if err != nil {
			slog.Warn("connector factory failed on startup", "url", c.URL, "err", err)
			continue
		}
		if err := m.toolkit.AddConnector(c.URL, conn); err != nil {
			slog.Warn("failed to add connector to toolkit on startup", "url", c.URL, "err", err)
			continue
		}
		slog.Debug("connector queued for connection", "url", c.URL, "namespace", types.Value(c.Namespace))
	}
}

// credOptsFor returns client opts that inject auth for the given URL.
// If the credential has a RefreshToken it installs an oauth2 transport that
// refreshes automatically; otherwise it falls back to a static bearer token.
func (m *Manager) credOptsFor(ctx context.Context, url string) []client.ClientOpt {
	if m.credentialStore == nil {
		return nil
	}
	cred, err := m.credentialStore.GetCredential(ctx, url)
	if err != nil || cred == nil || cred.Token == nil || cred.Token.AccessToken == "" {
		return nil
	}
	// Prefer a refreshing transport when we have a refresh token.
	if cred.RefreshToken != "" && cred.TokenURL != "" {
		return []client.ClientOpt{OAuthClientOpt(ctx, url, cred, m.credentialStore)}
	}
	return []client.ClientOpt{client.OptReqToken(client.Token{Scheme: "Bearer", Value: cred.Token.AccessToken})}
}

// onConnectorLog forwards log messages from a connector's MCP session to slog.
func (m *Manager) onConnectorLog(url string, level slog.Level, msg string, args ...any) {
	slog.Log(context.Background(), level, msg, append(args, "url", url)...)
}

// onConnectorState writes connector state back to the connector store.
// A zero ConnectedAt signals that the connector has disconnected.
func (m *Manager) onConnectorState(url string, state schema.ConnectorState) {
	_, _ = m.connectorStore.UpdateConnectorState(context.Background(), url, state)
	if state.ConnectedAt == nil || state.ConnectedAt.IsZero() {
		slog.Info("connector disconnected", "url", url)
	} else if name := types.Value(state.Name); name == "" {
		slog.Info("connector connected", "url", url)
	} else {
		slog.Info("connector connected", "url", url, "name", name, "version", types.Value(state.Version))
	}
}

// onConnectorTools logs when a connector's tool list changes.
func (m *Manager) onConnectorTools(url string, tools []llm.Tool) {
	if tools == nil {
		slog.Debug("connector tools cleared", "url", url)
	} else {
		slog.Info("connector tools updated", "url", url, "count", len(tools))
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
		return nil, llm.ErrNotFound.Withf("model %q not found in any provider", model)
	} else if client, ok := m.clients[provider]; !ok {
		return nil, llm.ErrNotFound.Withf("no client found for provider %q", provider)
	} else {
		return client.GetModel(ctx, model)
	}
}
