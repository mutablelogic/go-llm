package manager

import (
	"context"
	"log/slog"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	mcpclient "github.com/mutablelogic/go-llm/mcp/client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	google "github.com/mutablelogic/go-llm/provider/google"
	types "github.com/mutablelogic/go-server/pkg/types"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for configuring an agent
type Opt func(*Manager) error

// ConnectorFactory creates an llm.Connector for the given URL.
// The factory owns its static options (tracing, timeout, etc.).
// extraOpts are appended at call time and are used for per-URL dynamic options
// such as bearer token injection from a credential store.
type ConnectorFactory func(ctx context.Context, url string, extraOpts ...client.ClientOpt) (llm.Connector, error)

// WithLogger sets the logger used for connector state and diagnostic messages.
// If l is nil, slog.Default() is used.
func WithLogger(l *slog.Logger) Opt {
	return func(m *Manager) error {
		if l != nil {
			m.logger = l
		}
		return nil
	}
}

// MCPConnectorFactory returns a ConnectorFactory that creates MCP clients.
// The client auto-detects the transport (Streamable HTTP, falling back to SSE).
// name and version are reported to the server during the MCP initialisation handshake.
// staticOpts are captured at construction time and applied to every connector created.
func MCPConnectorFactory(name, version string, staticOpts ...mcpclient.Opt) ConnectorFactory {
	return func(ctx context.Context, url string, extraOpts ...client.ClientOpt) (llm.Connector, error) {
		opts := append([]mcpclient.Opt{mcpclient.WithClientOpt(extraOpts...)}, staticOpts...)
		c, err := mcpclient.New(url, name, version, opts...)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// AGENT OPTIONS

// WithClient adds an LLM client to the agent
func WithClient(client llm.Client) Opt {
	return func(m *Manager) error {
		if name := client.Name(); !types.IsIdentifier(name) {
			return schema.ErrBadParameter.Withf("invalid client name %q", name)
		} else if _, exists := m.clients[name]; exists {
			return schema.ErrBadParameter.Withf("duplicate client %q", name)
		} else {
			m.clients[name] = client
		}

		// Return success
		return nil
	}
}

// WithSessionStore sets the session storage backend for the manager.
// If not set, an in-memory store is used by default.
func WithSessionStore(store schema.SessionStore) Opt {
	return func(m *Manager) error {
		if store == nil {
			return schema.ErrBadParameter.With("session store is required")
		}
		m.sessionStore = store
		return nil
	}
}

// WithAgentStore sets the agent storage backend for the manager.
// If not set, an in-memory store is used by default.
func WithAgentStore(store schema.AgentStore) Opt {
	return func(m *Manager) error {
		if store == nil {
			return schema.ErrBadParameter.With("agent store is required")
		}
		m.agentStore = store
		return nil
	}
}

// WithConnectorStore sets the MCP connector storage backend for the manager.
// If not set, an in-memory store is used by default.
func WithConnectorStore(store schema.ConnectorStore) Opt {
	return func(m *Manager) error {
		if store == nil {
			return schema.ErrBadParameter.With("connector store is required")
		}
		m.connectorStore = store
		return nil
	}
}

// WithTools registers one or more tools with the manager's toolkit.
func WithTools(tools ...llm.Tool) Opt {
	return func(m *Manager) error {
		for _, t := range tools {
			if t == nil {
				return schema.ErrBadParameter.With("tool is required")
			}
		}
		m.toolkitOpts = append(m.toolkitOpts, tool.WithBuiltin(tools...))
		return nil
	}
}

// WithTool registers a single tool with the manager's toolkit.
func WithTool(t llm.Tool) Opt {
	return func(m *Manager) error {
		if t == nil {
			return schema.ErrBadParameter.With("tool is required")
		}
		m.toolkitOpts = append(m.toolkitOpts, tool.WithBuiltin(t))
		return nil
	}
}

// WithConnectorFactory sets the factory used to create MCP connectors.
// When set, CreateConnector probes the server before registering it.
// Use MCPConnectorFactory to get the standard MCP SSE implementation.
func WithConnectorFactory(factory ConnectorFactory) Opt {
	return func(m *Manager) error {
		if factory == nil {
			return schema.ErrBadParameter.With("connector factory is required")
		}
		m.connectorFactory = factory
		return nil
	}
}

// WithTracer sets the OpenTelemetry tracer for distributed tracing.
func WithTracer(tracer trace.Tracer) Opt {
	return func(m *Manager) error {
		m.tracer = tracer
		return nil
	}
}

// WithOutputDimensionality sets the output dimensionality for embedding requests
func WithOutputDimensionality(dim uint) opt.Opt {
	if dim == 0 {
		return opt.NoOp()
	}
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case "gemini":
			return google.WithOutputDimensionality(dim)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithOutputDimensionality not supported", provider))
		}
	})
}

// WithTitle sets the title for the agent, which may be used in embedding requests
func WithTitle(title string) opt.Opt {
	title = strings.TrimSpace(title)
	if title == "" {
		return opt.NoOp()
	}
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case "gemini":
			return google.WithTitle(title)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithTitle not supported", provider))
		}
	})
}

// WithTaskType sets the task type for embedding requests
func WithTaskType(taskType string) opt.Opt {
	taskType = strings.TrimSpace(taskType)
	if taskType == "" {
		return opt.NoOp()
	}
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case "gemini":
			return google.WithTaskType(taskType)
		default:
			return opt.Error(schema.ErrNotImplemented.Withf("%s: WithTaskType not supported", provider))
		}
	})
}
