package main

import (
	"fmt"
	"os"

	// Packages
	llmcmd "github.com/mutablelogic/go-llm/pkg/cmd"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	version "github.com/mutablelogic/go-server/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	llmcmd.ProviderCommands
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
