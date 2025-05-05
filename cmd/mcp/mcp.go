package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/mcp/server"
)

func main() {
	server := server.New("myserver", "0.1.0")
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintln(os.Stderr, "Running MCP server...")
	defer fmt.Fprintln(os.Stderr, "MCP server stopped", ctx.Err())
	if err := server.RunStdio(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(-1)
	}
}
