// Run an agent, which is the example agent which translates text, with a
// registry, and a store of agents, without a server:
//
//	go run ./examples/agent "Guten Morgen, wie geht's?"
//
// The agent replies with the translation, and the language of the text, as
// JSON, as its schema says. The provider is Ollama, when OLLAMA_ADDR is set,
// or OpenAI, when OPENAI_API_KEY is, as in the ask example. Eliza can't
// reply with JSON, so it doesn't work with this agent.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	agents "github.com/mutablelogic/go-llm/agents"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func main() {
	model := flag.String("model", "", "Model, by name, or by provider and name")
	language := flag.String("language", "", "Language to translate into, which is English when it isn't given")
	flag.Parse()
	text := strings.Join(flag.Args(), " ")
	if text == "" {
		text = "Guten Morgen, wie geht's?"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, *model, text, *language); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, model, text, language string) error {
	// A registry, with a store of agents, which is in memory, here, but which
	// a system would keep, such as in a database
	r, err := registry.New(registry.WithAgentStore(test.NewAgentStore()))
	if err != nil {
		return err
	}
	if model, err = addProvider(ctx, r, model); err != nil {
		return err
	}

	// The translate agent, from the examples, which has no model, so it's
	// given one, as it's created
	all, err := agents.All()
	if err != nil {
		return err
	}
	for _, meta := range all {
		if meta.Name != "translate" {
			continue
		}
		meta.Model = types.Ptr(model)
		if _, _, err := r.CreateAgent(ctx, meta); err != nil {
			return err
		}
	}

	// Run it, with its input, which its input schema validates, and which
	// its prompt is executed with
	input := map[string]string{"text": text}
	if language != "" {
		input["language"] = language
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	response, err := r.RunAgent(ctx, "translate", schema.RunAgentRequest{Input: data}, nil)
	if err != nil {
		return err
	}

	// Its reply, which is JSON, as its schema says
	var reply struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(response.Reply, &reply); err != nil {
		return fmt.Errorf("the reply isn't as the agent's schema says: %w", err)
	}
	fmt.Printf("%s\n\n(from %s, %d tokens in, %d out, in %v)\n", reply.Text, reply.Language, response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.Duration.Round(1e6))
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
