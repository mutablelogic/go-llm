package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/mcp/server"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

func main() {
	// Create tools
	toolkit := tool.NewToolKit()
	toolkit.Register(Weather{})

	// Create a new MCP server instance
	server, err := server.New("myserver", "0.1.0", server.WithToolKit(toolkit))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: ", err)
		os.Exit(-1)
	}

	// Cancel the server on interrupt or termination
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run the server
	if err := server.RunStdio(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error: ", err)
		os.Exit(-1)
	}
}

//////////////////////////////////////////////////////////////////////////////
// Create a tool

type Weather struct {
	City string
}

func (w Weather) Name() string {
	return "weather"
}

func (w Weather) Description() string {
	return "Return current weather information"
}

func (w Weather) Run(context.Context) (any, error) {
	return fmt.Sprintf("The weather in %s is sunny", w.City), nil
}
