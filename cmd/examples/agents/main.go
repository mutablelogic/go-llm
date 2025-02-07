package main

import (
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
	fmt.Println("Running agents are: ", agent.Name())
}
