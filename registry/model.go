package registry

import (
	"context"
	"errors"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The registry downloads and deletes models with their providers
var _ llm.Downloader = (*Registry)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// LoadModel downloads the model with the name, with its provider, which
// must download models, and returns it. As the model may not exist yet, the
// name must be its canonical name, such as ollama:llama3.2:latest, unless
// only one of the providers downloads models.
func (r *Registry) LoadModel(ctx context.Context, name string, progress llm.DownloadProgressFn) (_ llm.Model, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.LoadModel",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	if name == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}

	// A canonical name is its provider's name, and its name
	if provider, model, ok := strings.Cut(name, separator); ok {
		if c := r.cached(provider); c != nil {
			if _, ok := c.provider.Self().(llm.Downloader); !ok {
				return nil, llm.ErrNotImplemented.Withf("provider %q can't download models", provider)
			}
			return download(ctx, c, model, progress)
		}
	}

	// Otherwise, the one provider which downloads models
	var downloaders []*cached
	var names []string
	for _, e := range r.entries() {
		if _, ok := e.provider.Self().(llm.Downloader); ok {
			downloaders = append(downloaders, e.cache)
			names = append(names, e.name)
		}
	}
	switch len(downloaders) {
	case 0:
		return nil, llm.ErrNotImplemented.With("no provider can download models")
	case 1:
		return download(ctx, downloaders[0], name, progress)
	default:
		return nil, llm.ErrBadParameter.Withf("model %q needs its provider, as %s can download models, such as %s%s%s", name, strings.Join(names, " and "), names[0], separator, name)
	}
}

// DeleteModel deletes the model, with its provider, which must be in the
// registry, and download models
func (r *Registry) DeleteModel(ctx context.Context, model llm.Model) (err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.DeleteModel",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	downloader, err := capability[llm.Downloader](r, model, "delete models")
	if err != nil {
		return err
	}
	if err := downloader.DeleteModel(ctx, model); err != nil {
		return unavailable(err)
	}

	// The provider's models are listed again, without the model
	if name, ok := r.nameOf(model.Provider()); ok {
		if c := r.cached(name); c != nil {
			c.invalidate()
		}
	}
	return nil
}

// UnloadModel unloads the model from memory, with its provider, which must
// be in the registry, and load models, keeping it stored, so its provider's
// models aren't listed again. Returns ErrNotImplemented when the provider
// doesn't load models, or can't unload them.
func (r *Registry) UnloadModel(ctx context.Context, model llm.Model) (err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.UnloadModel",
		attribute.String("arg.model", r.modelName(model)),
	)
	defer func() { endSpan(err) }()

	downloader, err := capability[llm.Downloader](r, model, "unload models")
	if err != nil {
		return err
	}
	if err := downloader.UnloadModel(ctx, model); err != nil {
		return unavailable(err)
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// download downloads the model with the name, with the provider, whose
// models are listed again, with the model
func download(ctx context.Context, c *cached, name string, progress llm.DownloadProgressFn) (llm.Model, error) {
	model, err := c.provider.Self().(llm.Downloader).LoadModel(ctx, name, progress)
	if err != nil {
		return nil, unavailable(err)
	}
	c.invalidate()
	return model, nil
}

// unavailable returns the error of a provider as ErrServiceUnavailable,
// unless it's an llm.Err, such as ErrNotImplemented, keeping the provider's
// error, so a provider's response, such as its status, isn't the registry's
func unavailable(err error) error {
	if _, ok := errors.AsType[llm.Err](err); ok {
		return err
	}
	return llm.ErrServiceUnavailable.Withf("%w", err)
}

// modelName returns the canonical name of the model, for its span, or an
// empty string when there's no model
func (r *Registry) modelName(model llm.Model) string {
	if model == nil {
		return ""
	}
	return r.Name(model)
}

// capability returns the provider of the model, as T, which is what it can
// do, such as llm.Generator. Returns ErrNotFound when the model's provider
// isn't in the registry, and ErrNotImplemented when it can't do it, which
// the action says, such as "generate messages".
func capability[T any](r *Registry, model llm.Model, action string) (T, error) {
	var result T
	if model == nil {
		return result, llm.ErrBadParameter.With("model is required")
	}
	provider := model.Provider()
	if provider == nil {
		return result, llm.ErrBadParameter.Withf("model %q has no provider", model.Name())
	}
	name, ok := r.nameOf(provider)
	if !ok {
		return result, llm.ErrNotFound.Withf("provider %q of model %q isn't in the registry", provider.Name(), model.Name())
	}
	result, ok = provider.Self().(T)
	if !ok {
		return result, llm.ErrNotImplemented.Withf("provider %q can't %s", name, action)
	}
	return result, nil
}
