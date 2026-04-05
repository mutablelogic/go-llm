package cmd

import (
	"fmt"
	"os"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ConnectorCommands struct {
	ListConnectors  ListConnectorsCommand  `cmd:"" name:"connectors" help:"List connectors." group:"CONNECTOR"`
	CreateConnector CreateConnectorCommand `cmd:"" name:"connector-create" help:"Create a connector." group:"CONNECTOR"`
	DeleteConnector DeleteConnectorCommand `cmd:"" name:"connector-delete" help:"Delete a connector by URL." group:"CONNECTOR"`
	GetConnector    GetConnectorCommand    `cmd:"" name:"connector" help:"Get a connector by URL." group:"CONNECTOR"`
	UpdateConnector UpdateConnectorCommand `cmd:"" name:"connector-update" help:"Update connector metadata." group:"CONNECTOR"`
}

type ListConnectorsCommand struct {
	schema.ConnectorListRequest `embed:""`
}

type CreateConnectorCommand struct {
	URL                  string `arg:"" name:"url" help:"MCP server endpoint URL"`
	schema.ConnectorMeta `embed:""`
}

type DeleteConnectorCommand struct {
	URL string `arg:"" name:"url" help:"MCP server endpoint URL"`
}

type GetConnectorCommand struct {
	URL string `arg:"" name:"url" help:"MCP server endpoint URL"`
}

type UpdateConnectorCommand struct {
	URL                  string `arg:"" name:"url" help:"MCP server endpoint URL"`
	schema.ConnectorMeta `embed:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListConnectorsCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ListConnectorsCommand",
			attribute.String("request", types.Stringify(cmd.ConnectorListRequest)),
		)
		defer func() { endSpan(err) }()

		connectors, err := client.ListConnectors(parent, cmd.ConnectorListRequest)
		if err != nil {
			return err
		}

		if ctx.IsDebug() {
			fmt.Println(connectors)
			return nil
		}

		_, err = tui.TableFor[*schema.Connector]().Write(os.Stdout, connectors.Body...)
		return err
	})
}

func (cmd *CreateConnectorCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		req, err := cmd.request()
		if err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "CreateConnectorCommand",
			attribute.String("request", types.Stringify(req)),
		)
		defer func() { endSpan(err) }()

		connector, err := client.CreateConnector(parent, req)
		if err != nil {
			return err
		}

		fmt.Println(connector)
		return nil
	})
}

func (cmd *GetConnectorCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		if _, err := schema.CanonicalURL(cmd.URL); err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetConnectorCommand",
			attribute.String("url", cmd.URL),
		)
		defer func() { endSpan(err) }()

		connector, err := client.GetConnector(parent, cmd.URL)
		if err != nil {
			return err
		}

		fmt.Println(connector)
		return nil
	})
}

func (cmd *UpdateConnectorCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		req, err := cmd.request()
		if err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "UpdateConnectorCommand",
			attribute.String("url", cmd.URL),
			attribute.String("request", types.Stringify(req)),
		)
		defer func() { endSpan(err) }()

		connector, err := client.UpdateConnector(parent, cmd.URL, req)
		if err != nil {
			return err
		}

		fmt.Println(connector)
		return nil
	})
}

func (cmd CreateConnectorCommand) request() (schema.ConnectorInsert, error) {
	if _, err := schema.CanonicalURL(cmd.URL); err != nil {
		return schema.ConnectorInsert{}, err
	}

	return schema.ConnectorInsert{
		URL:           cmd.URL,
		ConnectorMeta: cmd.ConnectorMeta,
	}, nil
}

func (cmd UpdateConnectorCommand) request() (schema.ConnectorMeta, error) {
	if _, err := schema.CanonicalURL(cmd.URL); err != nil {
		return schema.ConnectorMeta{}, err
	}

	return cmd.ConnectorMeta, nil
}

func (cmd *DeleteConnectorCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		if _, err := schema.CanonicalURL(cmd.URL); err != nil {
			return err
		}

		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "DeleteConnectorCommand",
			attribute.String("url", cmd.URL),
		)
		defer func() { endSpan(err) }()

		connector, err := client.DeleteConnector(parent, cmd.URL)
		if err != nil {
			return err
		}

		fmt.Println(connector)
		return nil
	})
}
