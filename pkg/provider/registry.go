package provider

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	eliza "github.com/mutablelogic/go-llm/pkg/provider/eliza"
	gemini "github.com/mutablelogic/go-llm/pkg/provider/google"
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	ollama "github.com/mutablelogic/go-llm/pkg/provider/ollama"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Registry struct {
	mu          sync.RWMutex
	providers   map[string]provider
	clientopts  []client.ClientOpt
	regexpCache map[string]*regexp.Regexp
}

type provider struct {
	schema schema.Provider
	client llm.Client
	up     bool
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func New(opts ...client.ClientOpt) *Registry {
	self := new(Registry)
	self.providers = make(map[string]provider, 10)
	self.regexpCache = make(map[string]*regexp.Regexp, 10)
	self.clientopts = opts
	return self
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ping checks the connectivity of all providers and returns any errors
func (r *Registry) Ping(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result error
	for name, provider := range r.providers {
		up := true
		if err := provider.client.Ping(ctx); err != nil {
			result = errors.Join(result, schema.ErrServiceUnavailable.Withf("provider %q is unavailable: %v", name, err))
			up = false
		}
		if err := r.setUp(name, up); err != nil {
			result = errors.Join(result, err)
		}
	}

	return result
}

// Syncronizes the registry with the provided list of provider schemas and a decrypter function to obtain credentials.
// It returns lists of updated and deleted provider names, along with any errors encountered during the sync process.
func (r *Registry) Sync(schema []*schema.Provider, decrypter func(i int) (schema.ProviderCredentials, error)) (updates []string, deletes []string, err error) {
	var result error

	// Create a set of current provider names for easy lookup
	current := make(map[string]struct{}, len(r.providers))
	for name := range r.providers {
		current[name] = struct{}{}
	}

	// Iterate over the provided schemas and update or add providers as needed
	for i, s := range schema {
		credentials, err := decrypter(i)
		if err != nil {
			result = errors.Join(result, err)
		} else if updated, deleted, err := r.Set(s, credentials); err != nil {
			result = errors.Join(result, err)
		} else if updated {
			updates = append(updates, s.Name)
		} else if deleted {
			deletes = append(deletes, s.Name)
		}
		delete(current, s.Name) // Remove from current set to track which providers are still valid
	}

	// Any remaining providers in the current set are no longer present in the new schema and should be deleted
	for name := range current {
		delete(r.providers, name)
		deletes = append(deletes, name)
	}

	// Return any errors encountered during the sync process
	return updates, deletes, result
}

// Returns a provider client by name, or nil if not found.
func (r *Registry) Get(name string) llm.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if client, ok := r.providers[name]; ok {
		return client.client
	}
	return nil
}

// GetModels returns filtered models for a single provider using optional include/exclude regex patterns.
func (r *Registry) GetModels(ctx context.Context, name string, include, exclude []string) ([]schema.Model, error) {
	client := r.Get(name)
	if client == nil {
		return nil, schema.ErrNotFound.Withf("provider %q not found", name)
	}

	includePatterns, err := r.compiledModelPatterns(name, "include", include)
	if err != nil {
		return nil, err
	}
	excludePatterns, err := r.compiledModelPatterns(name, "exclude", exclude)
	if err != nil {
		return nil, err
	}

	models, err := client.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]schema.Model, 0, len(models))
	for _, model := range models {
		if len(includePatterns) > 0 && !matchesModelPattern(includePatterns, model.Name) {
			continue
		}
		if len(excludePatterns) > 0 && matchesModelPattern(excludePatterns, model.Name) {
			continue
		}
		if model.OwnedBy == "" {
			model.OwnedBy = name
		}
		result = append(result, model)
	}

	return result, nil
}

// Sets or updates a provider client by name, if the provider is enabled, and return boolean
// flags indicating whether the provider was updated or deleted.
func (r *Registry) Set(schema *schema.Provider, credentials schema.ProviderCredentials) (bool, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If the schema "enabled" field is false, delete the provider if it exists
	if schema.Enabled != nil && !types.Value(schema.Enabled) {
		delete(r.providers, schema.Name)
		return false, true, nil
	}

	// If the provider has been created but not modified, do not update the client
	existing, exists := r.providers[schema.Name]
	if exists && existing.schema.ModifiedAt == schema.ModifiedAt {
		// No update needed, return early
		return false, false, nil
	}

	// Create a new client for the provider
	client, err := createClient(schema, credentials, r.clientopts...)
	if err != nil {
		return false, false, err
	}

	// Update the registry with the new provider and client
	r.providers[schema.Name] = provider{
		schema: types.Value(schema),
		client: client,
	}

	// Return success
	return true, false, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func createClient(provider *schema.Provider, credentials schema.ProviderCredentials, opts ...client.ClientOpt) (llm.Client, error) {
	switch provider.Provider {
	case schema.Anthropic:
		if credentials.APIKey == "" {
			return nil, httpresponse.ErrBadRequest.Withf("missing API key for Anthropic provider")
		}
		return anthropic.New(credentials.APIKey, opts...)
	case schema.Ollama:
		return ollama.New(types.Value(provider.URL), opts...)
	case schema.Mistral:
		return mistral.New(credentials.APIKey, opts...)
	case schema.Gemini:
		return gemini.New(credentials.APIKey, opts...)
	case schema.Eliza:
		return eliza.New()
	}
	return nil, httpresponse.ErrBadRequest.Withf("unsupported provider: %s", provider.Provider)
}

// Syncronizes the registry with the provided list of provider schemas and a decrypter function to obtain credentials.
// It returns lists of updated and deleted provider names, along with any errors encountered during the sync process.
func (r *Registry) setUp(name string, value bool) error {
	provider, exists := r.providers[name]
	if !exists {
		return schema.ErrNotFound.Withf("provider %q not found", name)
	} else {
		provider.up = value
		r.providers[name] = provider
	}
	return nil
}

func (r *Registry) compiledModelPatterns(providerName, kind string, patterns []string) ([]*regexp.Regexp, error) {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for i, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Return regular expression from cache
		r.mu.RLock()
		re, exists := r.regexpCache[pattern]
		r.mu.RUnlock()
		if exists {
			result = append(result, re)
			continue
		}

		// Compile the regular expression and add to cache
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, schema.ErrBadParameter.Withf("provider %q %s[%d]: %v", providerName, kind, i, err)
		}

		r.mu.Lock()
		if re, exists = r.regexpCache[pattern]; !exists {
			r.regexpCache[pattern] = compiled
			re = compiled
		}
		r.mu.Unlock()

		result = append(result, re)
	}
	return result, nil
}

func matchesModelPattern(patterns []*regexp.Regexp, name string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}
