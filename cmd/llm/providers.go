package main

import (
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type AgentCommands struct {
	ListProviders ListProvidersCommand `cmd:"" name:"providers" help:"List available providers and their capabilities." group:"PROVIDER"`
}

type ListProvidersCommand struct{}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListProvidersCommand) Run(ctx *Globals) (err error) {
	agent, err := ctx.Agent()
	if err != nil {
		return err
	}

	// Get all clients
	clients := agent.Providers()

	// Print header
	fmt.Printf("%-20s %-12s %-12s %-12s\n", "AGENT", "MESSENGER", "EMBEDDER", "DOWNLOADER")
	fmt.Println("--------------------------------------------------------------------------------")

	// For each client, check interface implementations
	for _, client := range clients {
		messenger := "✗"
		embedder := "✗"
		downloader := "✗"

		// Check Messenger interface
		if _, ok := client.(llm.Messenger); ok {
			messenger = "✓"
		}

		// Check Embedder interface
		if _, ok := client.(llm.Embedder); ok {
			embedder = "✓"
		}

		// Check Downloader interface
		if _, ok := client.(llm.Downloader); ok {
			downloader = "✓"
		}

		fmt.Printf("%-20s %-12s %-12s %-12s\n", client.Name(), messenger, embedder, downloader)
	}

	return nil
}
