package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mutablelogic/go-llm/pkg/agent"
)

func main() {
	// Create a new agent which aggregates multiple providers
	agent, err := agent.New(
		agent.WithAnthropic(os.Getenv("ANTHROPIC_API_KEY")),
		agent.WithMistral(os.Getenv("MISTRAL_API_KEY")),
		agent.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
		agent.WithOllama(os.Getenv("OLLAMA_URL")),
	)
	if err != nil {
		panic(err)
	}

	// Get a model
	if len(os.Args) != 3 {
		fmt.Println("Usage: completion <model> <prompt>")
		os.Exit(-1)
	}

	model, err := agent.GetModel(context.TODO(), os.Args[1])
	if err != nil {
		panic(err)
	}

	// Get completion
	completion, err := model.Completion(context.TODO(), os.Args[2])
	if err != nil {
		panic(err)
	}

	fmt.Println("Completion is: ", completion)
}
