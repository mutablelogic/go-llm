//go:build !client

package main

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// newProvider is a provider which a new installation has, when its
// variable is set: an address, which is its URL, or a credential, which is
// its API key, as a placeholder, so the key isn't stored
type newProvider struct {
	variable string
	address  bool
	provider schema.Provider
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// newProviders are the providers which a new installation has, when their
// variables are set, as well as Eliza, which it always has, in the order
// they're created
var newProviders = []newProvider{
	{"OLLAMA_ADDR", true, schema.Provider{Name: "ollama", Type: llm.Ollama}},
	{"GOWHISPER_ADDR", true, schema.Provider{Name: "gowhisper", Type: llm.GoWhisper}},
	{"OPENAI_API_KEY", false, schema.Provider{Name: "openai", Type: llm.OpenAI}},
	{"DEEPSEEK_API_KEY", false, schema.Provider{Name: "deepseek", Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{URL: openai.DeepSeekEndpoint}}},
	{"MISTRAL_API_KEY", false, schema.Provider{Name: "mistral", Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{URL: openai.MistralEndpoint}}},
	{"ELEVENLABS_API_KEY", false, schema.Provider{Name: "elevenlabs", Type: llm.ElevenLabs}},
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// defaultProviders returns the providers of a new installation: Eliza, and
// those whose variables, which getenv returns, are set, with their
// addresses, or credentials, as placeholders, such as ${OPENAI_API_KEY}
func defaultProviders(getenv func(string) string) []schema.Provider {
	result := []schema.Provider{{Name: "eliza", Type: llm.Eliza}}
	for _, p := range newProviders {
		value := getenv(p.variable)
		if value == "" {
			continue
		}
		provider := p.provider
		if p.address {
			provider.URL = value
		} else {
			provider.APIKey = "${" + p.variable + "}"
		}
		result = append(result, provider)
	}
	return result
}

// createProviders creates the providers of a new installation, from the
// variables which getenv returns. A provider which can't be created, such as
// one which can't be reached, is logged, rather than stopping the server,
// and can be created later, with provider-create.
func createProviders(ctx server.Cmd, r *registry.Registry, getenv func(string) string) {
	for _, provider := range defaultProviders(getenv) {
		if _, err := r.CreateProvider(ctx.Context(), provider); err != nil {
			ctx.Logger().WarnContext(ctx.Context(), "provider not added", "provider", provider.Name, "error", err.Error())
		} else {
			ctx.Logger().InfoContext(ctx.Context(), "provider added", "provider", provider.Name)
		}
	}
}
