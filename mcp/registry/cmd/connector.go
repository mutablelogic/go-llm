package cmd

import (
	"context"
	"fmt"
	"os"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/mcp/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ConnectorCommands are the commands which list, get, create, update and
// delete the connectors, which are remote MCP servers
type ConnectorCommands struct {
	Connectors      ConnectorListCmd   `cmd:"" name:"connectors" help:"List the connectors, which are remote MCP servers, and whether each is connected." group:"MCP CONNECTORS"`
	Connector       ConnectorGetCmd    `cmd:"" name:"connector" help:"Get a connector by name." group:"MCP CONNECTORS"`
	ConnectorCreate ConnectorCreateCmd `cmd:"" name:"connector-create" help:"Create a connector to a remote MCP server, which the server connects to." group:"MCP CONNECTORS"`
	ConnectorUpdate ConnectorUpdateCmd `cmd:"" name:"connector-update" help:"Update a connector's URL, or patterns, so it's connected again." group:"MCP CONNECTORS"`
	ConnectorDelete ConnectorDeleteCmd `cmd:"" name:"connector-delete" help:"Delete a connector by name, so it's disconnected." group:"MCP CONNECTORS"`
}

type ConnectorListCmd struct{}

type ConnectorGetCmd struct {
	Name string `arg:"" name:"name" help:"Connector name."`
}

// ConnectorPatterns are the patterns of the names of a connector's
// server's tools, prompts and resources, as a provider's are of its models
type ConnectorPatterns struct {
	Include []string `name:"include" help:"Regular expressions of the names of the server's tools, prompts and resources which are included, or every one when there are none, such as ^search_."`
	Exclude []string `name:"exclude" help:"Regular expressions of the names of the server's tools, prompts and resources which are excluded, such as signup."`
}

type ConnectorCreateCmd struct {
	ConnectorPatterns
	Name string `arg:"" name:"name" help:"Connector name, which is an identifier, such as github."`
	URL  string `arg:"" name:"url" help:"URL of the MCP server, of its streamable HTTP, or SSE, transport, such as https://mcp.example.com/mcp."`
}

// ConnectorUpdateCmd updates a connector, whose patterns are replaced with
// those given, so one which isn't given is cleared, and whose URL is kept,
// unless one is given
type ConnectorUpdateCmd struct {
	ConnectorPatterns
	Name string `arg:"" name:"name" help:"Connector name."`
	URL  string `name:"url" help:"URL of the MCP server, which replaces it, or it's kept when it isn't given."`
}

type ConnectorDeleteCmd struct {
	Name string `arg:"" name:"name" help:"Connector name."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the connectors, as a table, and how many there are
func (cmd *ConnectorListCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "mcp.ConnectorListCmd", func(ctx context.Context, client *httpclient.Client) error {
		list, err := client.ListConnectors(ctx)
		if err != nil {
			return err
		} else if debug {
			fmt.Println(list)
			return nil
		}
		if _, err := tui.TableFor[*schema.Connector](tui.SetWidth(width)).Write(os.Stdout, list.Body...); err != nil {
			return err
		}
		_, err = tui.TableSummary("connectors", list.Count, uint(len(list.Body)), 0, nil).Write(os.Stdout)
		return err
	})
}

// Run prints the connector, with its status
func (cmd *ConnectorGetCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "mcp.ConnectorGetCmd", func(ctx context.Context, client *httpclient.Client) error {
		connector, err := client.GetConnector(ctx, cmd.Name)
		if err != nil {
			return err
		}
		fmt.Println(connector)
		return nil
	})
}

// Run creates the connector, and prints it
func (cmd *ConnectorCreateCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "mcp.ConnectorCreateCmd", func(ctx context.Context, client *httpclient.Client) error {
		connector, err := client.CreateConnector(ctx, schema.Connector{Name: cmd.Name, ConnectorMeta: schema.ConnectorMeta{URL: cmd.URL, Include: cmd.Include, Exclude: cmd.Exclude}})
		if err != nil {
			return err
		}
		fmt.Println(connector)
		return nil
	})
}

// Run updates the connector, and prints it
func (cmd *ConnectorUpdateCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "mcp.ConnectorUpdateCmd", func(ctx context.Context, client *httpclient.Client) error {
		url := cmd.URL
		if url == "" {
			existing, err := client.GetConnector(ctx, cmd.Name)
			if err != nil {
				return err
			}
			url = existing.URL
		}
		connector, err := client.UpdateConnector(ctx, cmd.Name, schema.ConnectorMeta{URL: url, Include: cmd.Include, Exclude: cmd.Exclude})
		if err != nil {
			return err
		}
		fmt.Println(connector)
		return nil
	})
}

// Run deletes the connector, and prints it, as it was
func (cmd *ConnectorDeleteCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "mcp.ConnectorDeleteCmd", func(ctx context.Context, client *httpclient.Client) error {
		connector, err := client.DeleteConnector(ctx, cmd.Name)
		if err != nil {
			return err
		}
		fmt.Println(connector)
		return nil
	})
}
