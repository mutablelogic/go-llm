package main

import (
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type AgentCommands struct {
	ListAgents ListAgentsCommand `cmd:"" name:"agents" help:"List available agents and their capabilities." group:"AGENT"`
}

type ListAgentsCommand struct{}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListAgentsCommand) Run(ctx *Globals) (err error) {
	agent, err := ctx.Agent()
	if err != nil {
		return err
	}

	// Get all clients
	clients := agent.Clients()

	// Print header
	fmt.Printf("%-20s %-12s %-12s %-12s\n", "AGENT", "MESSENGER", "EMBEDDER", "DOWNLOADER")
	fmt.Println("--------------------------------------------------------------------------------")

	// For each client, check interface implementations
	for name, client := range clients {
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

		fmt.Printf("%-20s %-12s %-12s %-12s\n", name, messenger, embedder, downloader)
	}

	return nil
}
