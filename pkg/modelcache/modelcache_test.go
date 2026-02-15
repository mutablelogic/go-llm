package modelcache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/modelcache"
	"github.com/mutablelogic/go-llm/pkg/opt"
	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/stretchr/testify/assert"
)

func makeModels(names ...string) []schema.Model {
	models := make([]schema.Model, len(names))
	for i, name := range names {
		models[i] = schema.Model{Name: name}
	}
	return models
}

func TestNewModelCache(t *testing.T) {
	assert := assert.New(t)
	mc := modelcache.NewModelCache(time.Hour, 10)
	assert.NotNil(mc)
}

func TestGetModel_FetchesAndCaches(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(time.Hour, 10)

	calls := 0
	fn := func(_ context.Context, name string) (*schema.Model, error) {
		calls++
		return &schema.Model{Name: name, Description: "desc"}, nil
	}

	// First call fetches from provider
	m, err := mc.GetModel(ctx, "model-a", fn)
	assert.NoError(err)
	assert.Equal("model-a", m.Name)
	assert.Equal("desc", m.Description)
	assert.Equal(1, calls)

	// Second call returns cached
	m, err = mc.GetModel(ctx, "model-a", fn)
	assert.NoError(err)
	assert.Equal("model-a", m.Name)
	assert.Equal(1, calls)
}

func TestGetModel_TTLExpiry(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(50*time.Millisecond, 10)

	calls := 0
	fn := func(_ context.Context, name string) (*schema.Model, error) {
		calls++
		return &schema.Model{Name: name}, nil
	}

	_, err := mc.GetModel(ctx, "model-a", fn)
	assert.NoError(err)
	assert.Equal(1, calls)

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	_, err = mc.GetModel(ctx, "model-a", fn)
	assert.NoError(err)
	assert.Equal(2, calls, "should re-fetch after TTL expiry")
}

func TestGetModel_NotFoundError(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(time.Hour, 10)

	fn := func(_ context.Context, name string) (*schema.Model, error) {
		return nil, llm.ErrNotFound
	}

	_, err := mc.GetModel(ctx, "missing", fn)
	assert.ErrorIs(err, llm.ErrNotFound)
}

func TestGetModel_NotFoundPrunesCache(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(1*time.Millisecond, 10)

	calls := 0
	fnOk := func(_ context.Context, name string) (*schema.Model, error) {
		calls++
		return &schema.Model{Name: name}, nil
	}
	fnNotFound := func(_ context.Context, name string) (*schema.Model, error) {
		return nil, llm.ErrNotFound
	}

	// Cache it
	_, err := mc.GetModel(ctx, "model-b", fnOk)
	assert.NoError(err)
	assert.Equal(1, calls)

	// Let it expire
	time.Sleep(5 * time.Millisecond)

	// Now return not found - should prune the entry
	_, err = mc.GetModel(ctx, "model-b", fnNotFound)
	assert.ErrorIs(err, llm.ErrNotFound)

	// Next call should fetch again
	_, err = mc.GetModel(ctx, "model-b", fnOk)
	assert.NoError(err)
	assert.Equal(2, calls)
}

func TestListModels_FetchesAndCaches(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(time.Hour, 10)

	calls := 0
	fn := func(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
		calls++
		return makeModels("zebra", "alpha", "middle"), nil
	}

	// First call fetches
	models, err := mc.ListModels(ctx, nil, fn)
	assert.NoError(err)
	assert.Len(models, 3)
	assert.Equal(1, calls)

	// Results should be sorted by name
	assert.Equal("alpha", models[0].Name)
	assert.Equal("middle", models[1].Name)
	assert.Equal("zebra", models[2].Name)

	// Second call returns cached
	models, err = mc.ListModels(ctx, nil, fn)
	assert.NoError(err)
	assert.Len(models, 3)
	assert.Equal(1, calls, "should not re-fetch within TTL")
}

func TestListModels_TTLExpiry(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(50*time.Millisecond, 10)

	calls := 0
	fn := func(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
		calls++
		return makeModels("model-1", "model-2"), nil
	}

	_, err := mc.ListModels(ctx, nil, fn)
	assert.NoError(err)
	assert.Equal(1, calls)

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	_, err = mc.ListModels(ctx, nil, fn)
	assert.NoError(err)
	assert.Equal(2, calls, "should re-fetch after TTL expiry")
}

func TestListModels_ReplacesCache(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(50*time.Millisecond, 10)

	calls := 0
	fn := func(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
		calls++
		if calls == 1 {
			return makeModels("model-a", "model-b", "model-c"), nil
		}
		return makeModels("model-x", "model-y"), nil
	}

	models, err := mc.ListModels(ctx, nil, fn)
	assert.NoError(err)
	assert.Len(models, 3)

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Should get the new set of models
	models, err = mc.ListModels(ctx, nil, fn)
	assert.NoError(err)
	assert.Len(models, 2)
	assert.Equal("model-x", models[0].Name)
	assert.Equal("model-y", models[1].Name)
}

func TestListModels_IndependentFromGetModel(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(time.Hour, 10)

	getModelCalls := 0
	getFn := func(_ context.Context, name string) (*schema.Model, error) {
		getModelCalls++
		return &schema.Model{Name: name}, nil
	}

	_, err := mc.GetModel(ctx, "individual-model", getFn)
	assert.NoError(err)
	assert.Equal(1, getModelCalls)

	// ListModels should still call provider
	listCalls := 0
	listFn := func(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
		listCalls++
		return makeModels("list-model-1", "list-model-2"), nil
	}

	models, err := mc.ListModels(ctx, nil, listFn)
	assert.NoError(err)
	assert.Equal(1, listCalls, "ListModels must call provider even if GetModel has cached entries")
	assert.Len(models, 2)
	assert.Equal("list-model-1", models[0].Name)
	assert.Equal("list-model-2", models[1].Name)
}

func TestListModels_GetModelUsesListCache(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(time.Hour, 10)

	listFn := func(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
		return makeModels("cached-1", "cached-2"), nil
	}
	_, err := mc.ListModels(ctx, nil, listFn)
	assert.NoError(err)

	getCalls := 0
	getFn := func(_ context.Context, name string) (*schema.Model, error) {
		getCalls++
		return &schema.Model{Name: name}, nil
	}

	m, err := mc.GetModel(ctx, "cached-1", getFn)
	assert.NoError(err)
	assert.Equal("cached-1", m.Name)
	assert.Equal(0, getCalls, "GetModel should use ListModels cache")
}

func TestListModels_Error(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(time.Hour, 10)

	fn := func(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
		return nil, fmt.Errorf("provider error")
	}

	models, err := mc.ListModels(ctx, nil, fn)
	assert.Error(err)
	assert.Nil(models)
	assert.Contains(err.Error(), "provider error")
}

func TestGetModel_Error(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(time.Hour, 10)

	fn := func(_ context.Context, name string) (*schema.Model, error) {
		return nil, fmt.Errorf("fetch error")
	}

	m, err := mc.GetModel(ctx, "bad-model", fn)
	assert.Error(err)
	assert.Nil(m)
}

func TestListModels_ZeroTTL(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	mc := modelcache.NewModelCache(0, 10)

	calls := 0
	fn := func(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
		calls++
		return makeModels("model-1"), nil
	}

	_, err := mc.ListModels(ctx, nil, fn)
	assert.NoError(err)
	assert.Equal(1, calls)

	// With zero TTL, should always re-fetch
	_, err = mc.ListModels(ctx, nil, fn)
	assert.NoError(err)
	assert.Equal(2, calls, "zero TTL should always re-fetch")
}
