package registry

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	"github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CachedClient struct {
	llm.Client
	mu     sync.Mutex
	ttl    time.Duration
	last   time.Time
	models map[string]schema.Model
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewCachedClient(client llm.Client, ttl time.Duration) *CachedClient {
	return &CachedClient{Client: client, ttl: ttl, models: make(map[string]schema.Model)}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Self returns the underlying client implementation.
func (c *CachedClient) Self() llm.Client {
	if c.Client == nil {
		return c
	}
	return c.Client.Self()
}

func (c *CachedClient) ListModels(ctx context.Context) ([]schema.Model, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Return cached models if not expired
	if c.cached() {
		return c.sortedModels(), nil
	}

	// Get models from client
	models, err := c.Client.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	// Cache models by name and set timestamp
	c.models = make(map[string]schema.Model, len(models))
	for _, model := range models {
		c.models[model.Name] = model
	}
	c.last = time.Now()

	// Return models sorted by name
	return c.sortedModels(), nil
}

// GetModel fails fast on cached misses but still defers successful lookups to the provider.
func (c *CachedClient) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	c.mu.Lock()
	cached := c.cached()
	_, ok := c.models[name]
	c.mu.Unlock()

	if cached && !ok {
		return nil, schema.ErrNotFound.Withf("model %q not found", name)
	}

	return c.Client.GetModel(ctx, name)
}

func (c *CachedClient) DeleteModel(ctx context.Context, model schema.Model) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if downloader, ok := c.Client.Self().(llm.Downloader); ok {
		if err := downloader.DeleteModel(ctx, model); err != nil {
			return err
		} else {
			c.last = time.Time{} // Invalidate cache
			delete(c.models, model.Name)
		}
		return nil
	}
	return schema.ErrNotImplemented.Withf("client does not support deleting models")
}

func (c *CachedClient) DownloadModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if downloader, ok := c.Client.Self().(llm.Downloader); ok {
		if model, err := downloader.DownloadModel(ctx, name, opts...); err != nil {
			return nil, err
		} else {
			c.last = time.Time{} // Invalidate cache
			return model, nil
		}
	}
	return nil, schema.ErrNotImplemented.Withf("client does not support downloading models")
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (c *CachedClient) cached() bool {
	return c.ttl > 0 && time.Since(c.last) < c.ttl
}

func (c *CachedClient) sortedModels() []schema.Model {
	return slices.SortedFunc(maps.Values(c.models), func(a, b schema.Model) int {
		return cmp.Compare(a.Name, b.Name)
	})
}
