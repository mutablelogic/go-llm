package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	// Packages
	kong "github.com/alecthomas/kong"
	client "github.com/mutablelogic/go-llm/pkg/mcp/client"
)

///////////////////////////////////////////////////////////////////////////////
// CLI

type CLI struct {
	Debug        bool   `name:"debug" short:"d" help:"Trace HTTP requests and responses to stderr"`
	ClientID     string `name:"client-id" help:"OAuth2 client ID"`
	ClientSecret string `name:"client-secret" help:"OAuth2 client secret"`
	CallbackPort int    `name:"callback-port" help:"Loopback port for OAuth callback (0 = random); ignored with --oob" default:"0"`
	OOB          bool   `name:"oob" help:"Out-of-band OAuth: browser displays the code, paste it into the terminal"`
	URL          string `arg:"" help:"MCP server URL" required:""`
}

///////////////////////////////////////////////////////////////////////////////
// MAIN

func main() {
	// Get execName
	var clientName string
	if exeName, err := os.Executable(); err != nil {
		fatalf("get executable name: %v", err)
	} else {
		clientName = filepath.Base(exeName)
	}

	var cli CLI
	kong.Parse(&cli,
		kong.Name(clientName),
		kong.Description("MCP client"),
		kong.UsageOnError(),
	)

	// Build client options.
	var opts []client.ClientOpt
	if cli.Debug {
		opts = append(opts, client.OptTrace(os.Stderr))
	}

	// Create the client.
	c, err := client.New(cli.URL, opts...)
	if err != nil {
		fatalf("create client: %v", err)
	}

	// Create context that is canceled on SIGINT/SIGTERM for graceful shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Connect to the server with optional OAuth authorization.
	if err := c.Connect(ctx, func(ctx context.Context) error {
		return c.Authorize(ctx, cli.URL, cli.ClientID, cli.ClientSecret, clientName, cli.CallbackPort, cli.OOB, func(authURL string) error {
			fmt.Printf("Opening browser for authorization...\n%s\n", authURL)
			return exec.Command("open", authURL).Start()
		})
	}); err != nil {
		fatalf("connect: %v", err)
	}
	defer c.Close()

	name, version, protocol := c.ServerInfo()
	fmt.Printf("Connected to %s %s (protocol %s)\n", name, version, protocol)

	// List tools (session is running in the background).
	tools, err := c.ListTools(ctx)
	if err != nil {
		fatalf("list tools: %v", err)
	}
	fmt.Printf("%d tool(s):\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  %-30s  %s\n", t.Name, t.Description)
	}

}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
