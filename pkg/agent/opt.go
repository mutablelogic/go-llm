package agent

import (
	"strings"

	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
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
		m.clients[client.Name()] = client
		return nil
	}
}

// WithSessionStore sets the session storage backend for the manager.
// If not set, session operations will return ErrNotImplemented.
func WithSessionStore(store schema.Store) Opt {
	return func(m *Manager) error {
		if store == nil {
			return llm.ErrBadParameter.With("session store is required")
		}
		m.store = store
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
