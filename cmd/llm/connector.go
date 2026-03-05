package main

import (
	"errors"
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	uitable "github.com/mutablelogic/go-llm/pkg/ui/table"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ConnectorCommands struct {
	ListConnectors  ListConnectorsCommand  `cmd:"" name:"connectors" help:"List registered MCP server connectors." group:"CONNECTOR"`
	GetConnector    GetConnectorCommand    `cmd:"" name:"connector" help:"Get a registered MCP server connector." group:"CONNECTOR"`
	AddConnector    AddConnectorCommand    `cmd:"" name:"add-connector" help:"Register a new MCP server connector." group:"CONNECTOR"`
	UpdateConnector UpdateConnectorCommand `cmd:"" name:"update-connector" help:"Update a registered MCP server connector." group:"CONNECTOR"`
	DeleteConnector DeleteConnectorCommand `cmd:"" name:"delete-connector" help:"Delete a registered MCP server connector." group:"CONNECTOR"`
}

type ListConnectorsCommand struct {
	Namespace string `name:"namespace" help:"Filter by namespace" optional:""`
	Enabled   *bool  `name:"enabled" help:"Filter by enabled state" optional:"" negatable:""`
	Limit     *uint  `name:"limit" help:"Maximum number of connectors to return" optional:""`
	Offset    uint   `name:"offset" help:"Offset for pagination" default:"0"`
}

type GetConnectorCommand struct {
	URL string `arg:"" name:"url" help:"MCP server connector URL"`
}

type AddConnectorCommand struct {
	URL       string  `arg:"" name:"url" help:"MCP server connector URL"`
	Namespace *string `name:"namespace" help:"Namespace prefix for tool disambiguation" optional:""`
	Enabled   bool    `name:"enabled" help:"Enable the connector immediately" default:"true" negatable:""`
}

type UpdateConnectorCommand struct {
	URL       string  `arg:"" name:"url" help:"MCP server connector URL"`
	Namespace *string `name:"namespace" help:"Namespace prefix" optional:""`
	Enabled   *bool   `name:"enabled" help:"Enable or disable the connector" optional:"" negatable:""`
}

type DeleteConnectorCommand struct {
	URL string `arg:"" name:"url" help:"MCP server connector URL"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListConnectorsCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ListConnectorsCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
	defer func() { endSpan(err) }()

	req := schema.ListConnectorsRequest{
		Namespace: cmd.Namespace,
		Enabled:   cmd.Enabled,
		Limit:     cmd.Limit,
		Offset:    cmd.Offset,
	}

	response, err := client.ListConnectors(parent, req)
	if err != nil {
		return err
	}

	// Print
	if ctx.Debug {
		fmt.Println(response)
	} else {
		if len(response.Body) > 0 {
			fmt.Println(uitable.Render(schema.ConnectorTable(response.Body)))
		}
		fmt.Println(TableSummary(len(response.Body), int(response.Offset), int(response.Count)))
	}
	return nil
}

func (cmd *GetConnectorCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "GetConnectorCommand",
		attribute.String("url", cmd.URL),
	)
	defer func() { endSpan(err) }()

	connector, err := client.GetConnector(parent, cmd.URL)
	if err != nil {
		return err
	}

	fmt.Println(connector)
	return nil
}

func (cmd *AddConnectorCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "AddConnectorCommand",
		attribute.String("url", cmd.URL),
	)
	defer func() { endSpan(err) }()

	connector, err := client.CreateConnector(parent, cmd.URL, schema.ConnectorMeta{
		Namespace: cmd.Namespace,
		Enabled:   types.Ptr(cmd.Enabled),
	})
	if err != nil {
		if errors.Is(err, httpresponse.ErrNotAuthorized) {
			return fmt.Errorf("%w\nhint: run 'llm login %s' to authenticate first", err, cmd.URL)
		}
		return err
	}

	fmt.Println(connector)
	return nil
}

func (cmd *UpdateConnectorCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "UpdateConnectorCommand",
		attribute.String("url", cmd.URL),
	)
	defer func() { endSpan(err) }()

	connector, err := client.UpdateConnector(parent, cmd.URL, schema.ConnectorMeta{
		Namespace: cmd.Namespace,
		Enabled:   cmd.Enabled,
	})
	if err != nil {
		return err
	}

	fmt.Println(connector)
	return nil
}

func (cmd *DeleteConnectorCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "DeleteConnectorCommand",
		attribute.String("url", cmd.URL),
	)
	defer func() { endSpan(err) }()

	if err := client.DeleteConnector(parent, cmd.URL); err != nil {
		return err
	}

	fmt.Printf("Deleted connector %s\n", cmd.URL)
	return nil
}
