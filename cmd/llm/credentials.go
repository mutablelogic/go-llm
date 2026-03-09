package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	// Packages
	server "github.com/mutablelogic/go-server"
	goclient "github.com/mutablelogic/go-client"
	oauth "github.com/mutablelogic/go-client/pkg/oauth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	mcpclient "github.com/mutablelogic/go-llm/pkg/mcp/client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
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
	Scopes            []string `name:"scope" help:"OAuth scopes to request"`
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
// COMMANDS

func (cmd *LoginCommand) Run(g server.Cmd) (err error) {
	var opts []goclient.ClientOpt
	if g.IsDebug() {
		opts = append(opts, goclient.OptTrace(os.Stderr, true))
	}

	// OTEL
	parent, endSpan := otel.StartSpan(g.Tracer(), g.Context(), "LoginCommand",
		attribute.String("url", cmd.URL),
	)
	defer func() { endSpan(err) }()

	// captured is set inside the authFn when OAuth succeeds.
	var captured *oauth.OAuthCredentials

	var c *mcpclient.Client
	c, err = mcpclient.New(cmd.URL, cmd.ClientName, "0", func(ctx context.Context, discoveryURL string) error {
		flow := c.OAuth()

		metadata, err := flow.Discover(ctx, discoveryURL)
		if err != nil {
			return fmt.Errorf("discover: %w", err)
		}

		switch {
		case cmd.ClientCredentials:
			// Machine-to-machine: client ID+secret required.
			if cmd.ClientID == "" {
				return fmt.Errorf("--client-id is required for --client-credentials flow")
			}
			if cmd.ClientSecret == "" {
				return fmt.Errorf("--client-secret is required for --client-credentials flow")
			}
			creds := &oauth.OAuthCredentials{
				ClientID:     cmd.ClientID,
				ClientSecret: cmd.ClientSecret,
				TokenURL:     metadata.TokenEndpoint,
				Metadata:     metadata,
			}
			captured, err = flow.AuthorizeWithCredentials(ctx, creds, cmd.Scopes...)

		case cmd.Device:
			// Device authorization grant (RFC 8628).
			creds, err := loginCreds(ctx, flow, metadata, cmd.ClientID, cmd.ClientSecret, cmd.ClientName)
			if err != nil {
				return err
			}
			captured, err = flow.AuthorizeWithDevice(ctx, creds, func(userCode, verificationURI string) error {
				fmt.Printf("Visit %s and enter code: %s\n", verificationURI, userCode)
				return nil
			}, cmd.Scopes...)

		default:
			// Browser-based Authorization Code + PKCE via loopback redirect.
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return fmt.Errorf("start callback listener: %w", err)
			}
			defer listener.Close()
			redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", listener.Addr().(*net.TCPAddr).Port)

			creds, err := loginBrowserCreds(ctx, flow, metadata, cmd.ClientID, cmd.ClientSecret, cmd.ClientName, redirectURI)
			if err != nil {
				return err
			}
			captured, err = flow.AuthorizeWithBrowser(ctx, creds, listener, func(authURL string) error {
				fmt.Printf("Opening browser for authorization...\n%s\n", authURL)
				return openBrowser(authURL)
			}, cmd.Scopes...)
		}
		return err
	}, opts...)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	// Drive the connect loop until the session is established (authFn fires on 401).
	connectCtx, connectCancel := context.WithCancel(parent)
	defer connectCancel()
	runErr := make(chan error, 1)
	go func() { runErr <- c.Run(connectCtx) }()
	for {
		_, err = c.ListTools(connectCtx)
		if err == nil {
			break
		}
		if !errors.Is(err, mcpclient.ErrNotConnected) {
			return fmt.Errorf("connect: %w", err)
		}
		select {
		case e := <-runErr:
			return fmt.Errorf("connect: %w", e)
		case <-connectCtx.Done():
			return connectCtx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	// Capture server info while the session is still live, then tear down.
	serverName, _, _ := c.ServerInfo()
	connectCancel()
	<-runErr // wait for Run to exit before proceeding

	if captured == nil {
		fmt.Printf("Server %s is accessible without authentication\n", cmd.URL)
		return nil
	}

	// Store credentials via the llm server.
	llmClient, err := clientFor(g)
	if err != nil {
		return err
	}
	if err := llmClient.SetCredential(parent, cmd.URL, schema.OAuthCredentials{
		Token:        captured.Token,
		ClientID:     captured.ClientID,
		ClientSecret: captured.ClientSecret,
		Endpoint:     cmd.URL,
		TokenURL:     captured.TokenURL,
	}); err != nil {
		return fmt.Errorf("store credential: %w", err)
	}

	fmt.Printf("Credentials stored for %s (%s)\n", cmd.URL, serverName)
	return nil
}

// loginCreds returns credentials for flows that don't need a redirect URI
// (device and client-credentials). Performs dynamic registration if no
// clientID is provided.
func loginCreds(ctx context.Context, flow interface {
	Register(context.Context, *oauth.OAuthMetadata, string, ...string) (*oauth.OAuthCredentials, error)
}, metadata *oauth.OAuthMetadata, clientID, clientSecret, clientName string) (*oauth.OAuthCredentials, error) {
	if clientID != "" {
		return &oauth.OAuthCredentials{ClientID: clientID, ClientSecret: clientSecret, TokenURL: metadata.TokenEndpoint, Metadata: metadata}, nil
	}
	if !metadata.SupportsRegistration() {
		return nil, fmt.Errorf("server does not support dynamic client registration; supply --client-id")
	}
	creds, err := flow.Register(ctx, metadata, clientName)
	if err != nil {
		return nil, fmt.Errorf("register client: %w", err)
	}
	return creds, nil
}

// loginBrowserCreds is like loginCreds but includes redirectURI for
// browser-based flows that require a loopback redirect.
func loginBrowserCreds(ctx context.Context, flow interface {
	Register(context.Context, *oauth.OAuthMetadata, string, ...string) (*oauth.OAuthCredentials, error)
}, metadata *oauth.OAuthMetadata, clientID, clientSecret, clientName, redirectURI string) (*oauth.OAuthCredentials, error) {
	if clientID != "" {
		return &oauth.OAuthCredentials{ClientID: clientID, ClientSecret: clientSecret, TokenURL: metadata.TokenEndpoint, Metadata: metadata}, nil
	}
	if !metadata.SupportsRegistration() {
		return nil, fmt.Errorf("server does not support dynamic client registration; supply --client-id")
	}
	creds, err := flow.Register(ctx, metadata, clientName, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("register client: %w", err)
	}
	return creds, nil
}

///////////////////////////////////////////////////////////////////////////////
// GET CREDENTIAL

func (cmd *GetCredentialCommand) Run(ctx server.Cmd) (err error) {
	client, err := clientFor(ctx)
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetCredentialCommand",
		attribute.String("url", cmd.URL),
	)
	defer func() { endSpan(err) }()

	// Get credential
	creds, err := client.GetCredential(parent, cmd.URL)
	if err != nil {
		return err
	}

	// Output credentials as JSON
	fmt.Println(types.Stringify(creds))
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// DELETE CREDENTIAL

func (cmd *DeleteCredentialCommand) Run(ctx server.Cmd) (err error) {
	client, err := clientFor(ctx)
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "DeleteCredentialCommand",
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
