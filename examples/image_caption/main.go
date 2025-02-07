package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mutablelogic/go-llm"
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

	// Check args
	if len(os.Args) != 3 {
		fmt.Println("Usage: image_caption <model> <filename>")
		os.Exit(-1)
	}

	// Get a model
	model, err := agent.GetModel(context.TODO(), os.Args[1])
	if err != nil {
		panic(err)
	}

	// Open file
	f, err := os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// Get image caption
	completion, err := model.Completion(context.TODO(), "Provide me with a description for this image", llm.WithAttachment(f))
	if err != nil {
		panic(err)
	}

	fmt.Println(completion.Text(0))
}
