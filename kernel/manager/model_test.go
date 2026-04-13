package manager

import (
	"context"
	"errors"
	"fmt"
	"testing"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	assert "github.com/stretchr/testify/assert"
)

type modelTestClient struct {
	name string
}

func (c *modelTestClient) Name() string { return c.name }

func (c *modelTestClient) Self() llm.Client { return c }

func (c *modelTestClient) Ping(context.Context) error { return nil }

func (c *modelTestClient) ListModels(context.Context) ([]schema.Model, error) {
	return nil, nil
}

func (c *modelTestClient) GetModel(context.Context, string) (*schema.Model, error) {
	return nil, nil
}

type modelTestDownloader struct {
	modelTestClient
}

var _ llm.Downloader = (*modelTestDownloader)(nil)

func (d *modelTestDownloader) Self() llm.Client { return d }

func (d *modelTestDownloader) DownloadModel(context.Context, string, ...opt.Opt) (*schema.Model, error) {
	return nil, nil
}

func (d *modelTestDownloader) DeleteModel(context.Context, schema.Model) error {
	return nil
}

func syncAndListModels(m *Manager, provider string, user *auth.User) func(context.Context) (*schema.ModelList, error) {
	return func(ctx context.Context) (*schema.ModelList, error) {
		if _, _, err := m.SyncProviders(ctx); err != nil {
			return nil, err
		}
		return m.ListModels(ctx, schema.ModelListRequest{Provider: provider}, user)
	}
}

func validateAccessibleModel(m *Manager, provider string, user *auth.User) func(context.Context, string) error {
	return func(ctx context.Context, name string) error {
		if _, _, err := m.SyncProviders(ctx); err != nil {
			return err
		}
		_, err := m.GetModel(ctx, schema.GetModelRequest{Provider: provider, Name: name}, user)
		return err
	}
}

func TestProviderAccessibleToUser(t *testing.T) {
	t.Run("public provider is accessible", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(providerAccessibleToUser(schema.Provider{Name: "public"}, &auth.User{}))
	})

	t.Run("grouped provider denied for user without groups", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(providerAccessibleToUser(schema.Provider{ProviderMeta: schema.ProviderMeta{Groups: []string{"admins"}}}, &auth.User{}))
	})

	t.Run("grouped provider allowed for matching group", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(providerAccessibleToUser(
			schema.Provider{ProviderMeta: schema.ProviderMeta{Groups: []string{"admins"}}},
			&auth.User{UserMeta: auth.UserMeta{Groups: []string{"admins"}}},
		))
	})

	t.Run("grouped provider denied for non-matching group", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(providerAccessibleToUser(
			schema.Provider{ProviderMeta: schema.ProviderMeta{Groups: []string{"admins"}}},
			&auth.User{UserMeta: auth.UserMeta{Groups: []string{"dev"}}},
		))
	})
}

func TestFilterProvidersForUser(t *testing.T) {
	assert := assert.New(t)
	providers := []schema.Provider{
		{Name: "public"},
		{Name: "admins", ProviderMeta: schema.ProviderMeta{Groups: []string{"admins"}}},
	}

	filtered := filterProvidersForUser(providers, &auth.User{})
	if assert.Len(filtered, 1) {
		assert.Equal("public", filtered[0].Name)
	}
}

func TestDownloaderCandidatesForProviders(t *testing.T) {
	assert := assert.New(t)
	providers := []schema.Provider{{Name: "plain"}, {Name: "downloader"}, {Name: "missing"}}
	clients := map[string]llm.Client{
		"plain":      &modelTestClient{name: "plain"},
		"downloader": &modelTestDownloader{modelTestClient{name: "downloader"}},
	}

	candidates := downloaderCandidatesForProviders(providers, func(name string) llm.Client {
		return clients[name]
	})

	if assert.Len(candidates, 1) {
		assert.Equal("downloader", candidates[0].provider.Name)
	}
}

func TestDeleteCandidatesForModels(t *testing.T) {
	assert := assert.New(t)
	downloaderA := &modelTestDownloader{modelTestClient{name: "provider-a"}}
	downloaderB := &modelTestDownloader{modelTestClient{name: "provider-b"}}
	candidates := []downloaderCandidate{
		{provider: schema.Provider{Name: "provider-a"}, downloader: downloaderA},
		{provider: schema.Provider{Name: "provider-b"}, downloader: downloaderB},
	}
	models := []schema.Model{
		{Name: "keep", OwnedBy: "provider-a"},
		{Name: "target", OwnedBy: "provider-a"},
		{Name: "target", OwnedBy: "provider-b"},
		{Name: "skip", OwnedBy: "provider-c"},
	}

	deletions := deleteCandidatesForModels([]schema.Model{models[1], models[2]}, candidates)
	if assert.Len(deletions, 2) {
		assert.Equal("provider-a", deletions[0].model.OwnedBy)
		assert.Equal("provider-b", deletions[1].model.OwnedBy)
	}

	deletions = deleteCandidatesForModels([]schema.Model{models[0]}, candidates)
	if assert.Len(deletions, 1) {
		assert.Equal("provider-a", deletions[0].model.OwnedBy)
	}

	assert.Empty(deleteCandidatesForModels([]schema.Model{{Name: "skip", OwnedBy: "provider-c"}}, candidates))
}

func TestIsModelNotFound(t *testing.T) {
	assert := assert.New(t)

	assert.True(isModelNotFound(schema.ErrNotFound))
	assert.True(isModelNotFound(schema.ErrNotFound.With("missing")))
	assert.True(isModelNotFound(httpresponse.ErrNotFound))
	assert.True(isModelNotFound(httpresponse.ErrNotFound.With("provider missing model")))
	assert.True(isModelNotFound(errors.Join(schema.ErrNotFound, context.Canceled)))

	assert.False(isModelNotFound(nil))
	assert.False(isModelNotFound(schema.ErrConflict))
	assert.False(isModelNotFound(httpresponse.ErrBadRequest.With("bad request")))
}

func TestIsIgnorableGetModelError(t *testing.T) {
	assert := assert.New(t)

	assert.True(isIgnorableGetModelError(schema.ErrNotFound.With("missing")))
	assert.True(isIgnorableGetModelError(httpresponse.ErrBadRequest.With("bad request")))
	assert.True(isIgnorableGetModelError(httpresponse.ErrNotAuthorized.With("missing credentials")))
	assert.True(isIgnorableGetModelError(httpresponse.ErrForbidden.With("forbidden")))
	assert.True(isIgnorableGetModelError(httpresponse.ErrConflict.With("conflict")))
	assert.True(isIgnorableGetModelError(fmt.Errorf("provider %q failed to get model %q: %w", "google-prod", "x/flux2-klein:latest", httpresponse.ErrBadRequest.With("unexpected model name format"))))
	assert.True(isIgnorableGetModelError(fmt.Errorf("provider %q failed to get model %q: %w", "mistral-primary", "x/flux2-klein:latest", schema.ErrBadParameter.With("unexpected model name format"))))

	assert.False(isIgnorableGetModelError(nil))
	assert.False(isIgnorableGetModelError(context.DeadlineExceeded))
	assert.False(isIgnorableGetModelError(httpresponse.ErrInternalError.With("boom")))
	assert.False(isIgnorableGetModelError(schema.ErrServiceUnavailable.With("retry later")))
}

func TestListModelsIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	admin := llmtest.AdminUser(conn)

	t.Run("matching group sees provider models", func(t *testing.T) {
		assert := assert.New(t)
		result, err := syncAndListModels(m, provider.Name, admin)(ctx)
		if llmtest.IsUnreachable(err) {
			t.Skipf("provider unreachable: %v", err)
		}
		if !assert.NoError(err) {
			return
		}
		assert.NotEmpty(result.Provider)
		assert.NotZero(result.Count)
		assert.NotEmpty(result.Body)
		assert.Contains(result.Provider, provider.Name)
	})

	t.Run("user without groups sees no provider models", func(t *testing.T) {
		assert := assert.New(t)
		result, err := syncAndListModels(m, provider.Name, &auth.User{})(ctx)
		if !assert.NoError(err) {
			return
		}
		assert.Empty(result.Provider)
		assert.Zero(result.Count)
		assert.Empty(result.Body)
	})
}

func TestGetModelIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	admin := llmtest.AdminUser(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, admin), nil, validateAccessibleModel(m, provider.Name, admin))

	t.Run("matching group gets model", func(t *testing.T) {
		assert := assert.New(t)
		result, err := m.GetModel(ctx, schema.GetModelRequest{Provider: provider.Name, Name: modelName}, admin)
		if llmtest.IsUnreachable(err) {
			t.Skipf("provider unreachable: %v", err)
		}
		if !assert.NoError(err) {
			return
		}
		assert.NotNil(result)
		assert.Equal(modelName, result.Name)
		assert.Equal(provider.Name, result.OwnedBy)
	})

	t.Run("user without groups gets not found", func(t *testing.T) {
		assert := assert.New(t)
		_, err := m.GetModel(ctx, schema.GetModelRequest{Provider: provider.Name, Name: modelName}, &auth.User{})
		if assert.Error(err) {
			assert.ErrorIs(err, schema.ErrNotFound)
		}
	})
}

func TestDownloadModelIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	admin := llmtest.AdminUser(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, admin), nil, validateAccessibleModel(m, provider.Name, admin))

	t.Run("matching group can download model", func(t *testing.T) {
		assert := assert.New(t)
		result, err := m.DownloadModel(ctx, schema.DownloadModelRequest{Provider: provider.Name, Name: modelName}, admin)
		if llmtest.IsUnreachable(err) {
			t.Skipf("provider unreachable: %v", err)
		}
		if !assert.NoError(err) {
			return
		}
		if assert.NotNil(result) {
			assert.Equal(modelName, result.Name)
			assert.Equal(provider.Name, result.OwnedBy)
		}
	})

	t.Run("user without groups gets not found", func(t *testing.T) {
		assert := assert.New(t)
		_, err := m.DownloadModel(ctx, schema.DownloadModelRequest{Provider: provider.Name, Name: modelName}, &auth.User{})
		if assert.Error(err) {
			assert.ErrorIs(err, schema.ErrNotFound)
		}
	})
}
