package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/agent"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////

type Adder struct {
	A float64 `name:"a" help:"The first number" required:"true"`
	B float64 `name:"b" help:"The second number" required:"true"`
}

func (Adder) Name() string {
	return "add_two_numbers"
}

func (Adder) Description() string {
	return "Add two numbers together and return the result"
}

func (a Adder) Run(context.Context) (any, error) {
	return a.A + a.B, nil
}

///////////////////////////////////////////////////////////////////////////////

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
	if len(os.Args) != 4 {
		fmt.Println("Usage: tool_adder <model> <first_num> <second_num>")
		os.Exit(-1)
	}

	// Get a model
	model, err := agent.GetModel(context.TODO(), os.Args[1])
	if err != nil {
		panic(err)
	}

	// Register tool
	toolkit := tool.NewToolKit()
	toolkit.Register(&Adder{})

	// Create a chat session
	session := model.Context(llm.WithToolKit(toolkit))

	// Make the prompt
	prompt := fmt.Sprintf("What is %v plus %v?", os.Args[2], os.Args[3])
	if err := session.FromUser(context.TODO(), prompt); err != nil {
		panic(err)
	}

	// Call tools
	for {
		calls := session.ToolCalls(0)
		if len(calls) == 0 {
			break
		}

		// Get the results from the toolkit
		fmt.Println("Running", calls)
		results, err := toolkit.Run(context.TODO(), calls...)
		if err != nil {
			panic(err)
		}

		// Get another tool call or a user response
		if err := session.FromTool(context.TODO(), results...); err != nil {
			panic(err)
		}
	}

	// Return the result
	fmt.Println(session.Text(0))
}
