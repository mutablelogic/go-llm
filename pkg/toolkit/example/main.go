package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/toolkit"
)

func main() {
	// Create a toolkit with builtins and a handler for connector events and prompt execution.
	h := NewHandler()
	tk, err := toolkit.New(
		toolkit.WithHandler(h),
	)
	if err != nil {
		log.Fatal(err)
	}
	h.SetToolkit(tk)

	// Add a remote MCP connector — namespace inferred from the server.
	// Can be called before or while Run is active.
	if err = tk.AddConnector("https://remote.mcpservers.org/fetch/mcp"); err != nil {
		log.Fatal(err)
	}

	// Or provide an explicit namespace.
	if err = tk.AddConnectorNS("my-server", "https://remote.mcpservers.org/sequentialthinking/mcp"); err != nil {
		log.Fatal(err)
	}

	// Run until CTRL-C.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Run starts all connectors and blocks until ctx is cancelled.
	// It closes the toolkit and waits for all connectors to finish on return.
	// Connectors can be added and removed while Run is active.
	if err = tk.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
