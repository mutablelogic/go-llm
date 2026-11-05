//go:build !client

package main

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_DefaultProviders(t *testing.T) {
	// Eliza, always
	assert.Equal(t, []schema.Provider{{Name: "eliza", Type: llm.Eliza}}, defaultProviders(func(string) string { return "" }))

	// And those whose variables are set, with addresses as URLs, and
	// credentials as placeholders, so keys aren't stored
	env := map[string]string{
		"OLLAMA_ADDR":      "http://ollama:11434",
		"OPENAI_API_KEY":   "sk-secret",
		"MISTRAL_API_KEY":  "secret",
		"UNRELATED_SECRET": "x",
	}
	providers := defaultProviders(func(name string) string { return env[name] })
	assert.Equal(t, []schema.Provider{
		{Name: "eliza", Type: llm.Eliza},
		{Name: "ollama", Type: llm.Ollama, ProviderMeta: schema.ProviderMeta{URL: "http://ollama:11434"}},
		{Name: "openai", Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{APIKey: "${OPENAI_API_KEY}"}},
		{Name: "mistral", Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{URL: openai.MistralEndpoint, APIKey: "${MISTRAL_API_KEY}"}},
	}, providers)
	for _, provider := range providers {
		assert.NotContains(t, provider.APIKey, "secret")
	}
}
