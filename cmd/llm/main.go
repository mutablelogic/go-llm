package main

import (
	"fmt"
	"os"

	// Packages
	version "github.com/mutablelogic/go-llm/pkg/version"
	cmd "github.com/mutablelogic/go-server/pkg/cmd"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	AgentCommands
	ConnectorCommands
	CredentialsCommands
	GenerateCommands
	ModelCommands
	SessionCommands
	ToolCommands
	ServerCommands
	TelegramCommands
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func main() {
	if err := cmd.Main(CLI{}, "llm command line interface", version.Version()); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(-1)
	}
}
