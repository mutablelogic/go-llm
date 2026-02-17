package main

import (
	"encoding/json"
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	"go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CredentialsCommands struct {
	Login LoginCommand `cmd:"" name:"login" help:"Authenticate with an MCP server." group:"CREDENTIALS"`
}

type LoginCommand struct {
	URL          string   `arg:"" name:"url" help:"MCP server URL (e.g., https://mcp.asana.com/sse)"`
	ClientID     string   `name:"client-id" help:"OAuth client ID (auto-registers if not provided and server supports it)" default:""`
	ClientSecret string   `name:"client-secret" help:"OAuth client secret (for machine-to-machine)" default:""`
	Scopes       []string `name:"scope" help:"OAuth scopes to request" default:""`
	Device       bool     `name:"device" help:"Use device authorization flow instead of interactive" default:"false"`
	ClientName   string   `name:"client-name" help:"Client name for dynamic registration" default:"${EXECUTABLE_NAME}"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *LoginCommand) Run(ctx *Globals) (err error) {
	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "LoginCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
	defer func() { endSpan(err) }()

	// Get the client
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// Determine which flow to use
	var creds *schema.OAuthCredentials

	switch {
	case cmd.ClientSecret != "":
		// Machine-to-machine: Client Credentials flow
		if cmd.ClientID == "" {
			return fmt.Errorf("client-id is required for machine-to-machine authentication")
		}
		creds, err = client.ClientCredentialsLogin(parent, cmd.URL, cmd.ClientID, cmd.ClientSecret, cmd.Scopes)
		if err != nil {
			return fmt.Errorf("client credentials login failed: %w", err)
		}

	case cmd.Device:
		// Device Authorization flow
		creds, err = client.DeviceLogin(parent, cmd.URL, cmd.ClientID, cmd.ClientName, cmd.Scopes, func(verificationURI, userCode string) {
			ctx.logger.Printf(parent, "To authenticate, visit: %s", verificationURI)
			ctx.logger.Printf(parent, "Enter code: %s", userCode)
		})
		if err != nil {
			return fmt.Errorf("device login failed: %w", err)
		}

	default:
		// Create a listener for the callback and start the interactive login flow
		listener, _, err := httpclient.NewCallbackListener("")
		if err != nil {
			return err
		}
		defer listener.Close()

		// Interactive: Authorization Code with PKCE
		creds, err = client.InteractiveLogin(parent, cmd.URL, cmd.ClientID, cmd.ClientName, cmd.Scopes, listener, func(authURL string) {
			ctx.logger.Printf(parent, "Open this URL in your browser to authenticate:")
			ctx.logger.Printf(parent, "%s", authURL)
		})
		if err != nil {
			return fmt.Errorf("interactive login failed: %w", err)
		}
	}

	// Output credentials as JSON
	output, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(output))

	return nil
}
