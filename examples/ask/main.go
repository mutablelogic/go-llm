// Ask a model a question, and stream its reply, with a registry of
// providers, without a server:
//
//	go run ./examples/ask "Explain a KV cache in two sentences"
//
// The provider is Ollama, when OLLAMA_ADDR is set, such as localhost:11434,
// or OpenAI, when OPENAI_API_KEY is, or otherwise Eliza, which needs
// neither. The model is set with -model.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func main() {
	model := flag.String("model", "", "Model, by name, or by provider and name")
	flag.Parse()
	question := strings.Join(flag.Args(), " ")
	if question == "" {
		question = "Explain a KV cache in two sentences"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, *model, question); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, model, question string) error {
	// A registry, with a provider, from the environment
	r, err := registry.New()
	if err != nil {
		return err
	}
	if model, err = addProvider(ctx, r, model); err != nil {
		return err
	}

	// Ask the model, with a system prompt, and stream its reply
	response, err := r.Ask(ctx, schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{
			Model:  types.Ptr(model),
			System: types.Ptr("Be brief"),
		},
		Messages: []providerschema.MessageMeta{
			{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(question)}}},
		},
	}, func(what llm.StreamType, text string) {
		if what == llm.StreamText {
			fmt.Print(text)
		}
	})
	if err != nil {
		return err
	}
	fmt.Printf("\n\n%s: %d tokens in, %d out, in %v\n", response.Model, response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.Duration.Round(1e6))
	return nil
}

// addProvider adds a provider to the registry, which is Ollama, when
// OLLAMA_ADDR is set, or OpenAI, when OPENAI_API_KEY is, or Eliza, and
// returns the model, which is the one given, or the provider's default
func addProvider(ctx context.Context, r *registry.Registry, model string) (string, error) {
	var provider schema.Provider
	var fallback string
	switch {
	case os.Getenv("OLLAMA_ADDR") != "":
		provider = schema.Provider{Type: llm.Ollama, ProviderMeta: schema.ProviderMeta{URL: os.Getenv("OLLAMA_ADDR")}}
		fallback = "llama3.2"
	case os.Getenv("OPENAI_API_KEY") != "":
		provider = schema.Provider{Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{APIKey: "${OPENAI_API_KEY}"}}
		fallback = "gpt-4o-mini"
	default:
		provider = schema.Provider{Type: llm.Eliza}
		fallback = "eliza-1966-en"
	}
	if _, err := r.CreateProvider(ctx, provider); err != nil {
		return "", err
	}
	if model == "" {
		model = fallback
	}
	return model, nil
}
