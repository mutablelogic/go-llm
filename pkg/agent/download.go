package agent

import (
	"context"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// DownloadModel downloads the specified model using a path with provider scheme.
// path should be in the format "provider:model" (e.g., "ollama:llama2").
// Returns ErrBadParameter if path format is invalid.
// Returns ErrNotFound if the provider doesn't exist.
// Returns ErrNotImplemented if the provider doesn't support downloading.
func (a *agent) DownloadModel(ctx context.Context, path string, opts ...opt.Opt) (*schema.Model, error) {
	// Parse the path to extract provider and model name
	providerName, modelPath, err := parsePath(path)
	if err != nil {
		return nil, err
	}

	// Find the client by provider name
	client, ok := a.clients[providerName]
	if !ok {
		return nil, llm.ErrNotFound.Withf("provider %q not found", providerName)
	}

	// Check if client implements Downloader
	downloader, ok := client.(llm.Downloader)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("provider %q does not support downloading models", providerName)
	}

	// Download the model
	return downloader.DownloadModel(ctx, modelPath, opts...)
}

// DeleteModel deletes the specified model from local storage.
// Returns ErrNotFound if no client owns the model.
// Returns ErrNotImplemented if the provider doesn't support deleting.
func (a *agent) DeleteModel(ctx context.Context, model schema.Model) error {
	// Find the client that owns this model
	client := a.clientForModel(model)
	if client == nil {
		return llm.ErrNotFound.With("no client found for model")
	}

	// Check if client implements Downloader
	downloader, ok := client.(llm.Downloader)
	if !ok {
		return llm.ErrNotImplemented.Withf("provider %q does not support deleting models", client.Name())
	}

	// Delete the model
	return downloader.DeleteModel(ctx, model)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// parsePath parses a path in the format "provider:model" and returns the provider and model parts.
// Returns ErrBadParameter if the path format is invalid.
func parsePath(path string) (provider, modelPath string, err error) {
	parts := strings.SplitN(path, ":", 2)
	if len(parts) != 2 {
		return "", "", llm.ErrBadParameter.Withf("invalid path format %q, expected \"provider:model\"", path)
	}

	provider = strings.TrimSpace(parts[0])
	modelPath = strings.TrimSpace(parts[1])

	if provider == "" {
		return "", "", llm.ErrBadParameter.With("provider name cannot be empty")
	}
	if modelPath == "" {
		return "", "", llm.ErrBadParameter.With("model path cannot be empty")
	}

	return provider, modelPath, nil
}
