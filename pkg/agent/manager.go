package agent

import (
	"context"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	session "github.com/mutablelogic/go-llm/pkg/session"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Manager struct {
	clients map[string]llm.Client
	store   schema.Store
	toolkit *tool.Toolkit
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewManager(opts ...Opt) (*Manager, error) {
	// Create the manager
	m := new(Manager)
	m.clients = make(map[string]llm.Client)

	// Apply options
	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, err
		}
	}

	// Default to in-memory session store if none was provided
	if m.store == nil {
		m.store = session.NewMemoryStore()
	}

	// Default to empty toolkit if none was provided
	if m.toolkit == nil {
		m.toolkit, _ = tool.NewToolkit()
	}

	// Return success
	return m, nil
}

func (m *Manager) Close() error {
	// Close is a no-op
	return nil
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
