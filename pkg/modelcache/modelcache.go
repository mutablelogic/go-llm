package modelcache

import (
	"context"
	"errors"
	"sort"
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
	ttl   time.Duration
	model map[string]modelts
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
	self.model = make(map[string]modelts, cap)

	// Return the model cache
	return self
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (mc *ModelCache) GetModel(ctx context.Context, name string, fn GetModelFunc) (*schema.Model, error) {
	// Cached model
	if entry, ok := mc.model[name]; ok {
		if time.Since(entry.ts) < mc.ttl {
			return types.Ptr(entry.model), nil
		}
		// Expired entry: prune before fetching
		delete(mc.model, name)
	}

	// Fetch model
	model, err := fn(ctx, name)
	if err == nil {
		mc.model[model.Name] = modelts{ts: time.Now(), model: types.Value(model)}
	} else {
		// If model no longer exists, ensure cache is invalidated
		if errors.Is(err, llm.ErrNotFound) {
			delete(mc.model, name)
		}
		return nil, err
	}

	// Return model
	return model, err
}

func (mc *ModelCache) ListModels(ctx context.Context, opts []opt.Opt, fn ListModelsFunc) ([]schema.Model, error) {
	// If we have a TTL and cached entries, return all non-expired models
	if mc.ttl > 0 && len(mc.model) > 0 {
		now := time.Now()
		cached := make([]schema.Model, 0, len(mc.model))
		for name, entry := range mc.model {
			if now.Sub(entry.ts) < mc.ttl {
				cached = append(cached, entry.model)
			} else {
				// Prune expired entries
				delete(mc.model, name)
			}
		}
		if len(cached) > 0 {
			sort.Slice(cached, func(i, j int) bool { return cached[i].Name < cached[j].Name })
			return cached, nil
		}
	}

	// Fetch models
	models, err := fn(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Cache models
	now := time.Now()
	for _, model := range models {
		mc.model[model.Name] = modelts{ts: now, model: model}
	}

	// Sort models by name
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })

	// Return sorted list of models
	return models, nil
}
