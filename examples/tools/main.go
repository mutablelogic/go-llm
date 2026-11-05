// A tool of your own, which a model calls, with a toolkit, and a registry
// which runs the tools a model calls, until it replies, without a server:
//
//	go run ./examples/tools "Roll two dice, and tell me what I got"
//
// The provider is Ollama, when OLLAMA_ADDR is set, or OpenAI, when
// OPENAI_API_KEY is, as in the ask example, whose model must be able to
// call tools. Eliza can't, so it doesn't call the tool.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// A TOOL

// roll is a tool, which rolls dice
type roll struct {
	toolkit.Base
}

// rollInput is the input of the tool, whose JSON schema the model is told
type rollInput struct {
	Dice  int `json:"dice" help:"How many dice to roll" min:"1" max:"10"`
	Sides int `json:"sides,omitempty" help:"How many sides each die has, which is six when it isn't given" optional:""`
}

func (roll) Name() string {
	return "roll"
}

func (roll) Description() string {
	return "Rolls dice, and returns what each die rolled, and their total."
}

func (roll) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[rollInput]()
}

// Run rolls the dice, saying what it's doing, which is streamed, as the
// model replies, and returns the rolls, which the model is sent
func (roll) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req rollInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.Sides == 0 {
		req.Sides = 6
	}
	toolkit.Logf(ctx, "Rolling %d dice of %d sides", req.Dice, req.Sides)
	rolls := make([]int, req.Dice)
	total := 0
	for i := range rolls {
		rolls[i] = rand.IntN(req.Sides) + 1
		total += rolls[i]
	}
	return map[string]any{"rolls": rolls, "total": total}, nil
}

///////////////////////////////////////////////////////////////////////////////
// MAIN

func main() {
	model := flag.String("model", "", "Model, by name, or by provider and name, which can call tools")
	flag.Parse()
	question := strings.Join(flag.Args(), " ")
	if question == "" {
		question = "Roll two dice, and tell me what I got"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, *model, question); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, model, question string) error {
	// A toolkit, with the tool, in a namespace, with a system prompt which
	// says how to use it, and a registry, which models call its tools from
	tools, err := toolkit.New(ctx)
	if err != nil {
		return err
	}
	if err := tools.Add(toolkit.NewNamespace("dice", "Roll dice with the roll tool, rather than making up a roll.", roll{})); err != nil {
		return err
	}
	r, err := registry.New(registry.WithToolkit(tools))
	if err != nil {
		return err
	}
	if model, err = addProvider(ctx, r, model); err != nil {
		return err
	}

	// Ask the model, in a session, which isn't stored, with the tools in the
	// namespace, which the registry runs, as the model calls them, until it
	// replies, streaming what the tools log, and the reply
	session := schema.Session{SessionMeta: schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{
		Model: types.Ptr(model),
		Tools: []string{"dice:*"},
	}}}
	message := &providerschema.Message{MessageMeta: providerschema.MessageMeta{
		Sender:  "user",
		Content: []providerschema.ContentBlock{{Text: types.Ptr(question)}},
	}}
	response, err := r.WithSession(ctx, session, []llm.Message{message}, func(what llm.StreamType, text string) {
		switch what {
		case llm.StreamLog:
			fmt.Println("[" + text + "]")
		case llm.StreamText:
			fmt.Print(text)
		}
	})
	if err != nil {
		return err
	}

	// What was added: the question, the tool calls, their results, and the
	// reply, and the tokens used
	fmt.Printf("\n\n%d messages, %d tokens in, %d out, in %v\n", len(response.Messages), response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.Duration.Round(1e6))
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
