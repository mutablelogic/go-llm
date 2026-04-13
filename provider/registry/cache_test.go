package registry

import (
	"context"
	"errors"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

type cachedListClient struct {
	models    []schema.Model
	getModels map[string]schema.Model
	calls     int
	gets      int
}

func (c *cachedListClient) Name() string { return "cached-list" }

func (c *cachedListClient) Self() llm.Client { return c }

func (c *cachedListClient) Ping(context.Context) error { return nil }

func (c *cachedListClient) ListModels(context.Context) ([]schema.Model, error) {
	c.calls++
	return append([]schema.Model(nil), c.models...), nil
}

func (c *cachedListClient) GetModel(_ context.Context, name string) (*schema.Model, error) {
	c.gets++
	if model, ok := c.getModels[name]; ok {
		copy := model
		return &copy, nil
	}
	for _, model := range c.models {
		if model.Name == name {
			copy := model
			return &copy, nil
		}
	}
	return nil, schema.ErrNotFound.Withf("model %q not found", name)
}

func TestCachedClientListModelsCachesByName(t *testing.T) {
	client := &cachedListClient{models: []schema.Model{{Name: "zeta"}, {Name: "alpha"}, {Name: "alpha", Description: "replacement"}}}
	cache := NewCachedClient(client, time.Hour)

	models, err := cache.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 {
		t.Fatalf("expected one provider call, got %d", client.calls)
	}
	if len(models) != 2 {
		t.Fatalf("expected duplicate names to collapse to 2 models, got %d", len(models))
	}
	if models[0].Name != "alpha" || models[1].Name != "zeta" {
		t.Fatalf("expected sorted cached models, got %+v", models)
	}
	if models[0].Description != "replacement" {
		t.Fatalf("expected latest model for duplicate name, got %+v", models[0])
	}

	models, err = cache.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 {
		t.Fatalf("expected cached response on second call, got %d provider calls", client.calls)
	}
	if len(models) != 2 {
		t.Fatalf("expected cached models on second call, got %d", len(models))
	}
}

func TestCachedClientGetModelUsesProviderOnCachedHit(t *testing.T) {
	client := &cachedListClient{
		models: []schema.Model{{Name: "alpha", Description: "cached-list"}},
		getModels: map[string]schema.Model{
			"alpha": {Name: "alpha", Description: "provider-get"},
		},
	}
	cache := NewCachedClient(client, time.Hour)

	if _, err := cache.ListModels(context.Background()); err != nil {
		t.Fatal(err)
	}

	model, err := cache.GetModel(context.Background(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if model == nil || model.Description != "provider-get" {
		t.Fatalf("expected provider GetModel result, got %+v", model)
	}
	if client.gets != 1 {
		t.Fatalf("expected cached hit to call provider GetModel once, got %d", client.gets)
	}
}

func TestCachedClientGetModelFailsFastOnCachedMiss(t *testing.T) {
	client := &cachedListClient{models: []schema.Model{{Name: "alpha"}}}
	cache := NewCachedClient(client, time.Hour)

	if _, err := cache.ListModels(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := cache.GetModel(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected missing model error")
	}
	if !errors.Is(err, schema.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if client.gets != 0 {
		t.Fatalf("expected cached miss to avoid provider GetModel call, got %d", client.gets)
	}
}

func TestCachedClientGetModelFallsBackWithoutCache(t *testing.T) {
	client := &cachedListClient{models: []schema.Model{{Name: "alpha", Description: "provider"}}}
	cache := NewCachedClient(client, time.Hour)

	model, err := cache.GetModel(context.Background(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if model == nil || model.Description != "provider" {
		t.Fatalf("expected provider model, got %+v", model)
	}
	if client.gets != 1 {
		t.Fatalf("expected provider GetModel call without cache, got %d", client.gets)
	}
}
