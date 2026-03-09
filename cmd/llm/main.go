package main

import (
	"fmt"
	"os"

	// Packages
	cmd "github.com/mutablelogic/go-server/pkg/cmd"
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	AgentCommands
	ConnectorCommands
	CredentialsCommands
	GenerateCommands
	MCPCommands
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
