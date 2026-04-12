package main

import (
	"fmt"
	"os"

	// Packages
	llmcmd "github.com/mutablelogic/go-llm/kernel/cmd"
	mcpcmd "github.com/mutablelogic/go-llm/mcp/cmd"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	llmcmd.SessionCommands
	llmcmd.ChatCommands
	llmcmd.ChannelCommands
	llmcmd.AskCommands
	llmcmd.EmbeddingCommands
	llmcmd.ConnectorCommands
	llmcmd.ProviderCommands
	llmcmd.ModelCommands
	llmcmd.ToolCommands
	llmcmd.AgentCommands
	MCP mcpcmd.Commands `cmd:"" name:"mcp" help:"Interact directly with an MCP server." group:"MCP"`
	ServerCommands
}

type ServerCommands struct {
	RunServer llmcmd.RunServer `cmd:"" name:"run" help:"Run the server." group:"SERVER"`
	servercmd.OpenAPICommands
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const description = "LLM Server provides an interface for managing large language model interactions."

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func main() {
	if err := servercmd.Main(CLI{}, description, version.Version()); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(-1)
	}
}
