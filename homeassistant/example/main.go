package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	// Packages
	kong "github.com/alecthomas/kong"
	homeassistant "github.com/mutablelogic/go-llm/homeassistant/connector"
	mcpserver "github.com/mutablelogic/go-llm/mcp/server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

const (
	serverName         = "homeassistant"
	serverVersion      = "0.1.0"
	serverTitle        = "Home Assistant MCP Server"
	serverInstructions = "Use these tools to inspect and control a Home Assistant instance."
)

type CLI struct {
	Endpoint string `help:"Home Assistant base URL." env:"HOME_ASSISTANT_URL" required:""`
	APIKey   string `help:"Home Assistant long-lived access token." env:"HOME_ASSISTANT_API_KEY" required:""`
	Listen   string `help:"HTTP listen address." default:":8080"`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func main() {
	cli := CLI{}
	parser, err := kong.New(&cli,
		kong.Name(serverName),
		kong.Description(serverInstructions),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	_, err = parser.Parse(os.Args[1:])
	if err != nil {
		parser.FatalIfErrorf(err)
	}

	if err := run(cli); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(cli CLI) error {
	conn, err := homeassistant.New(cli.Endpoint, cli.APIKey)
	if err != nil {
		return err
	}

	srv, err := mcpserver.New(
		serverName,
		serverVersion,
		mcpserver.WithTitle(serverTitle),
		mcpserver.WithInstructions(serverInstructions),
	)
	if err != nil {
		return err
	}
	if err := srv.AddConnector(context.Background(), conn); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", srv.Handler())
	server := http.Server{
		Addr:    cli.Listen,
		Handler: mux,
	}

	log.Printf("Home Assistant MCP server listening on http://%s/", cli.Listen)
	return server.ListenAndServe()
}
