package manager

import (
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for configuring an agent
type Opt func(*Manager) error

///////////////////////////////////////////////////////////////////////////////
// AGENT OPTIONS

// WithClient adds an LLM client to the agent
func WithClient(client llm.Client) Opt {
	return func(m *Manager) error {
		if name := client.Name(); !types.IsIdentifier(name) {
			return llm.ErrBadParameter.Withf("invalid client name %q", name)
		} else if _, exists := m.clients[name]; exists {
			return llm.ErrBadParameter.Withf("duplicate client %q", name)
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
			return llm.ErrBadParameter.With("session store is required")
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
			return llm.ErrBadParameter.With("agent store is required")
		}
		m.agentStore = store
		return nil
	}
}

// WithToolkit sets the toolkit for the manager.
func WithToolkit(toolkit *tool.Toolkit) Opt {
	return func(m *Manager) error {
		if toolkit == nil {
			return llm.ErrBadParameter.With("toolkit is required")
		}
		m.toolkit = toolkit
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
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithOutputDimensionality not supported", provider))
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
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithTitle not supported", provider))
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
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithTaskType not supported", provider))
		}
	})
}
