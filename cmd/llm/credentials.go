package main

import (
	"context"
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
	var token interface{}

	switch {
	case cmd.ClientSecret != "":
		// Machine-to-machine: Client Credentials flow
		if cmd.ClientID == "" {
			return fmt.Errorf("client-id is required for machine-to-machine authentication")
		}
		token, err = client.ClientCredentialsLogin(parent, cmd.URL, cmd.ClientID, cmd.ClientSecret, cmd.Scopes)
		if err != nil {
			return fmt.Errorf("client credentials login failed: %w", err)
		}

	case cmd.Device:
		// Device Authorization flow
		// First check if server supports device flow before registration
		metadata, err := client.DiscoverOAuth(parent, cmd.URL)
		if err != nil {
			return err
		}
		if !metadata.SupportsDeviceFlow() {
			return fmt.Errorf("%s does not support device authorization flow", cmd.URL)
		}

		clientID := cmd.ClientID
		if clientID == "" {
			clientID, err = cmd.autoRegister(parent, client, ctx, metadata, nil)
			if err != nil {
				return err
			}
		}
		token, err = client.DeviceLogin(parent, cmd.URL, clientID, cmd.Scopes, func(verificationURI, userCode string) {
			ctx.logger.Printf(parent, "To authenticate, visit: %s", verificationURI)
			ctx.logger.Printf(parent, "Enter code: %s", userCode)
		})
		if err != nil {
			return fmt.Errorf("device login failed: %w", err)
		}

	default:
		// Interactive: Authorization Code with PKCE
		// Create listener to get redirect URI for registration
		listener, redirectURI, err := httpclient.NewCallbackListener("")
		if err != nil {
			return err
		}
		defer listener.Close()

		clientID := cmd.ClientID
		if clientID == "" {
			clientID, err = cmd.autoRegister(parent, client, ctx, nil, []string{redirectURI})
			if err != nil {
				return err
			}
		}
		token, err = client.InteractiveLogin(parent, cmd.URL, clientID, cmd.Scopes, listener, func(authURL string) {
			ctx.logger.Printf(parent, "Open this URL in your browser to authenticate:")
			ctx.logger.Printf(parent, "%s", authURL)
		})
		if err != nil {
			return fmt.Errorf("interactive login failed: %w", err)
		}
	}

	// Output token as JSON
	output, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(output))

	return nil
}

// autoRegister attempts to dynamically register a client if the server supports it.
// If metadata is nil, it will be discovered from the server.
func (cmd *LoginCommand) autoRegister(parent context.Context, client *httpclient.Client, ctx *Globals, metadata *schema.OAuthMetadata, redirectURIs []string) (string, error) {
	// Discover OAuth metadata if not provided
	if metadata == nil {
		var err error
		metadata, err = client.DiscoverOAuth(parent, cmd.URL)
		if err != nil {
			return "", err
		}
	}

	if !metadata.SupportsRegistration() {
		return "", fmt.Errorf("%s does not support dynamic client registration; use --client-id to provide a pre-registered OAuth client ID", cmd.URL)
	}

	ctx.logger.Printf(parent, "Registering client '%s'...", cmd.ClientName)
	clientInfo, err := client.RegisterClient(parent, metadata, cmd.ClientName, redirectURIs)
	if err != nil {
		return "", fmt.Errorf("dynamic client registration failed (you may need to register manually and use --client-id): %w", err)
	}
	ctx.logger.Printf(parent, "Registered client ID: %s", clientInfo.ClientID)
	return clientInfo.ClientID, nil
}
