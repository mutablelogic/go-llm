package agent

import (
	"context"
	"sort"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *Manager) ListModels(ctx context.Context, req schema.ListModelsRequest) (*schema.ListModelsResponse, error) {
	var mu sync.Mutex
	var all []schema.Model

	// Collect models from all clients in parallel
	wg, ctx := errgroup.WithContext(ctx)
	var matched bool
	for _, client := range m.clients {
		// Match the provider option (skip filter if empty)
		if req.Provider != "" && client.Name() != req.Provider {
			continue
		}
		matched = true

		// Fetch in parallel and aggregate results
		wg.Go(func() error {
			models, err := client.ListModels(ctx)
			if err != nil {
				return err
			}

			mu.Lock()
			defer mu.Unlock()
			all = append(all, models...)
			return nil
		})
	}
	if err := wg.Wait(); err != nil {
		return nil, err
	}

	// Check if provider filter matched
	if req.Provider != "" && !matched {
		return nil, llm.ErrNotFound.Withf("provider %q not found", req.Provider)
	}

	// Sort all models by name
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })

	// Paginate
	total := uint(len(all))
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + types.Value(req.Limit)
	if req.Limit == nil || end > total {
		end = total
	}

	// Append provider name
	provider := make([]string, 0, len(m.clients))
	for name := range m.clients {
		provider = append(provider, name)
	}

	// Return success
	return &schema.ListModelsResponse{
		Count:    total,
		Offset:   req.Offset,
		Limit:    req.Limit,
		Provider: provider,
		Body:     all[start:end],
	}, nil
}

func (m *Manager) GetModel(ctx context.Context, req schema.GetModelRequest) (*schema.Model, error) {
	var mu sync.Mutex
	var result *schema.Model

	// Provide cancelable context to short-circuit once we find the model
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Search all clients for the model in parallel (filtered by provider if specified)
	wg, ctx := errgroup.WithContext(ctx)
	var matched bool
	for _, client := range m.clients {
		// Match the provider option (skip filter if empty)
		if req.Provider != "" && client.Name() != req.Provider {
			continue
		}
		matched = true

		wg.Go(func() error {
			model, err := client.GetModel(ctx, req.Name)
			if err != nil {
				return nil // Swallow per-provider not-found errors
			}

			mu.Lock()
			defer mu.Unlock()
			if result == nil {
				result = model
				cancel() // Short-circuit remaining lookups
			}
			return nil
		})
	}

	// Return any errors (or not found if result is nil)
	if err := wg.Wait(); err != nil {
		return nil, err
	}
	if req.Provider != "" && !matched {
		return nil, llm.ErrNotFound.Withf("provider %q not found", req.Provider)
	}
	if result == nil {
		return nil, llm.ErrNotFound.Withf("model '%s' not found", req.Name)
	}

	// Return success
	return result, nil
}
