package main

import (
	"fmt"
	"os"

	// Packages
	llm "github.com/mutablelogic/go-llm/kernel/cmd2"
	mcpcmd "github.com/mutablelogic/go-llm/mcp/cmd"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	/*	llm.SessionCommands
		llm.ChatCommands
		llm.ChannelCommands
		llm.AskCommands
		llm.EmbeddingCommands
		llm.ConnectorCommands
		llm.ProviderCommands
		llm.ModelCommands
		llm.ToolCommands
		llm.AgentCommands
	*/
	MCP mcpcmd.Commands `cmd:"" name:"mcp" help:"Interact directly with an MCP server." group:"MCP"`
	ServerCommands
}

type ServerCommands struct {
	RunServer llm.RunServer `cmd:"" name:"run" help:"Run the server." group:"SERVER"`
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
