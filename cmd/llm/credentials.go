package main

import (
	"encoding/json"
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	types "github.com/mutablelogic/go-server/pkg/types"
	"go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CredentialsCommands struct {
	Login            LoginCommand            `cmd:"" name:"login" help:"Authenticate with an MCP server and store credentials." group:"CREDENTIALS"`
	GetCredential    GetCredentialCommand    `cmd:"" name:"credentials" help:"Get stored credentials for an MCP server." group:"CREDENTIALS"`
	DeleteCredential DeleteCredentialCommand `cmd:"" name:"delete-credentials" help:"Delete stored credentials for an MCP server." group:"CREDENTIALS"`
}

type LoginCommand struct {
	URL               string   `arg:"" name:"url" help:"MCP server URL (e.g., https://mcp.asana.com/sse)"`
	ClientID          string   `name:"client-id" help:"OAuth client ID (auto-registers if not provided and server supports it)" default:""`
	ClientSecret      string   `name:"client-secret" help:"OAuth client secret (for confidential clients)" default:""`
	Scopes            []string `name:"scope" help:"OAuth scopes to request" default:"openid"`
	Device            bool     `name:"device" help:"Use device authorization flow" default:"false"`
	ClientCredentials bool     `name:"client-credentials" help:"Use client credentials (machine-to-machine) flow" default:"false"`
	ClientName        string   `name:"client-name" help:"Client name for dynamic registration" default:"${EXECUTABLE_NAME}"`
}

type GetCredentialCommand struct {
	URL string `arg:"" name:"url" help:"MCP server URL"`
}

type DeleteCredentialCommand struct {
	URL string `arg:"" name:"url" help:"MCP server URL"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (cmd LoginCommand) String() string {
	v := cmd
	if v.ClientSecret != "" {
		v.ClientSecret = "***"
	}
	return types.Stringify(v)
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *LoginCommand) Run(ctx *Globals) error {
	return fmt.Errorf("OAuth login is not implemented")
}

///////////////////////////////////////////////////////////////////////////////
// GET CREDENTIAL

func (cmd *GetCredentialCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "GetCredentialCommand",
		attribute.String("url", cmd.URL),
	)
	defer func() { endSpan(err) }()

	// Get credential
	creds, err := client.GetCredential(parent, cmd.URL)
	if err != nil {
		return err
	}

	// Output credentials as JSON
	output, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(output))

	return nil
}

///////////////////////////////////////////////////////////////////////////////
// DELETE CREDENTIAL

func (cmd *DeleteCredentialCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "DeleteCredentialCommand",
		attribute.String("url", cmd.URL),
	)
	defer func() { endSpan(err) }()

	// Delete credential
	if err := client.DeleteCredential(parent, cmd.URL); err != nil {
		return err
	}

	fmt.Printf("Deleted credentials for %s\n", cmd.URL)
	return nil
}
