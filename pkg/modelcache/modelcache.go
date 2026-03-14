package modelcache

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type modelts struct {
	ts    time.Time
	model schema.Model
}

type ModelCache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	cap   int
	model map[string]modelts
	ts    time.Time // timestamp of last ListModels fetch
}

type GetModelFunc func(context.Context, string) (*schema.Model, error)
type ListModelsFunc func(context.Context, ...opt.Opt) ([]schema.Model, error)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewModelCache(ttl time.Duration, cap int) *ModelCache {
	self := new(ModelCache)

	// Set the TTL for each model
	if ttl > 0 {
		self.ttl = ttl
	}

	// Set model cache capacity
	self.cap = cap
	self.model = make(map[string]modelts, cap)

	// Return the model cache
	return self
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (mc *ModelCache) GetModel(ctx context.Context, name string, fn GetModelFunc) (*schema.Model, error) {
	// Check cache under read lock
	mc.mu.RLock()
	entry, ok := mc.model[name]
	mc.mu.RUnlock()

	if ok && time.Since(entry.ts) < mc.ttl {
		return types.Ptr(entry.model), nil
	}

	// Entry absent or expired: remove stale entry and fetch fresh
	if ok {
		mc.mu.Lock()
		delete(mc.model, name)
		mc.mu.Unlock()
	}

	// Fetch model
	model, err := fn(ctx, name)
	if err == nil {
		mc.mu.Lock()
		mc.model[model.Name] = modelts{ts: time.Now(), model: types.Value(model)}
		mc.mu.Unlock()
	} else {
		// If model no longer exists, ensure cache is invalidated
		if errors.Is(err, llm.ErrNotFound) {
			mc.mu.Lock()
			delete(mc.model, name)
			mc.mu.Unlock()
		}
		return nil, err
	}

	// Return model
	return model, err
}

func (mc *ModelCache) ListModels(ctx context.Context, opts []opt.Opt, fn ListModelsFunc) ([]schema.Model, error) {
	// If the list was fetched recently, return cached entries
	mc.mu.RLock()
	if mc.ttl > 0 && !mc.ts.IsZero() && time.Since(mc.ts) < mc.ttl {
		cached := make([]schema.Model, 0, len(mc.model))
		for _, entry := range mc.model {
			cached = append(cached, entry.model)
		}
		mc.mu.RUnlock()
		sort.Slice(cached, func(i, j int) bool { return cached[i].Name < cached[j].Name })
		return cached, nil
	}
	mc.mu.RUnlock()

	// Fetch models from provider
	models, err := fn(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Replace cache with fresh list
	now := time.Now()
	newMap := make(map[string]modelts, len(models))
	for _, model := range models {
		newMap[model.Name] = modelts{ts: now, model: model}
	}
	mc.mu.Lock()
	mc.ts = now
	mc.model = newMap
	mc.mu.Unlock()

	// Sort models by name
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })

	// Return sorted list of models
	return models, nil
}

// Flush clears all cached model entries, forcing the next read to fetch from the provider.
func (mc *ModelCache) Flush() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ts = time.Time{}
	mc.model = make(map[string]modelts, mc.cap)
}
