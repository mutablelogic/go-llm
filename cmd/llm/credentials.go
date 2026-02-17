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
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CredentialsCommands struct {
	Login LoginCommand `cmd:"" name:"login" help:"Authenticate with an MCP server." group:"CREDENTIALS"`
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

	// Build the OAuth2 config
	cfg := &oauth2.Config{
		ClientID:     cmd.ClientID,
		ClientSecret: cmd.ClientSecret,
		Scopes:       cmd.Scopes,
		Endpoint:     oauth2.Endpoint{AuthURL: cmd.URL},
	}

	// Build login options
	loginOpts := []httpclient.LoginOpt{
		httpclient.OptClientName(cmd.ClientName),
	}

	// Determine which flow to use
	var creds *schema.OAuthCredentials

	switch {
	case cmd.ClientCredentials:
		// Machine-to-machine: Client Credentials flow
		if cmd.ClientID == "" {
			return fmt.Errorf("--client-id is required for client credentials flow")
		}
		if cmd.ClientSecret == "" {
			return fmt.Errorf("--client-secret is required for client credentials flow")
		}
		creds, err = client.Login(parent, cfg, append(loginOpts, httpclient.OptClientCredentials())...)
		if err != nil {
			return fmt.Errorf("client credentials login failed: %w", err)
		}

	case cmd.Device:
		// Device Authorization flow
		creds, err = client.Login(parent, cfg, append(loginOpts, httpclient.OptDevice(func(verificationURI, userCode string) {
			ctx.logger.Printf(parent, "To authenticate, visit: %s", verificationURI)
			ctx.logger.Printf(parent, "Enter code: %s", userCode)
		}))...)
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

		// Interactive: Authorization Code with PKCE (optional client secret for confidential clients)
		creds, err = client.Login(parent, cfg, append(loginOpts, httpclient.OptInteractive(listener, func(authURL string) {
			ctx.logger.Printf(parent, "Open this URL in your browser to authenticate:")
			ctx.logger.Printf(parent, "%s", authURL)
		}))...)
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
