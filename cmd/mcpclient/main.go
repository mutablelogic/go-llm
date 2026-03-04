package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	// Packages
	kong "github.com/alecthomas/kong"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	goclient "github.com/mutablelogic/go-client"
	oauth "github.com/mutablelogic/go-client/pkg/oauth"
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
	var opts []goclient.ClientOpt
	if cli.Debug {
		opts = append(opts, goclient.OptTrace(os.Stderr, true))
	}

	// Create context that is canceled on SIGINT/SIGTERM for graceful shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create the client, wiring in the OAuth flow for auth-protected servers.
	var c *client.Client
	var err error
	c, err = client.New(cli.URL, clientName, "0", func(ctx context.Context, discoveryURL string) error {
		return authorize(ctx, c, discoveryURL, cli.ClientID, cli.ClientSecret, clientName, cli.CallbackPort, cli.OOB)
	}, opts...)
	if err != nil {
		fatalf("create client: %v", err)
	}

	// Run drives the full connect→session→teardown lifecycle in the background.
	runErr := make(chan error, 1)
	go func() { runErr <- c.Run(ctx) }()

	// Wait until the session is established or Run fails.
	var tools []*sdkmcp.Tool
	for {
		tools, err = c.ListTools(ctx)
		if err == nil {
			break
		}
		if !errors.Is(err, client.ErrNotConnected) {
			fatalf("list tools: %v", err)
		}
		select {
		case e := <-runErr:
			fatalf("connect: %v", e)
		case <-ctx.Done():
			fatalf("connect: %v", ctx.Err())
		default:
			runtime.Gosched()
		}
	}

	name, version, protocol := c.ServerInfo()
	fmt.Printf("Connected to %s %s (protocol %s)\n", name, version, protocol)
	fmt.Printf("%d tool(s):\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  %-30s  %s\n", t.Name, t.Description)
	}

}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// authorize performs the OAuth 2.0 discovery, (optional) dynamic registration,
// and authorization flow, storing the resulting token on the client so the
// next Connect attempt can succeed.
func authorize(ctx context.Context, c *client.Client, discoveryURL, clientID, clientSecret, clientName string, callbackPort int, oob bool) error {
	flow := c.OAuth()

	// Step 1: discover the authorization server endpoints (RFC 8414 / RFC 9728).
	metadata, err := flow.Discover(ctx, discoveryURL)
	if err != nil {
		return fmt.Errorf("discover authorization server: %w", err)
	}

	if oob {
		// OOB flow: register without a redirect URI, then prompt the user to
		// visit the authorization URL and paste back the code.
		creds, err := buildCreds(ctx, flow, metadata, clientID, clientSecret, clientName)
		if err != nil {
			return err
		}
		_, err = flow.AuthorizeWithCode(ctx, creds, func(authURL string) (string, error) {
			_ = exec.Command("open", authURL).Start()
			fmt.Printf("Open this URL to authorize:\n%s\n\nEnter the verification code: ", authURL)
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			return strings.TrimSpace(scanner.Text()), scanner.Err()
		})
		return err
	}

	// Browser flow: open a loopback listener first so we know the redirect URI
	// before registering the client.
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", callbackPort))
	if err != nil {
		return fmt.Errorf("start callback listener: %w", err)
	}
	defer listener.Close()
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", listener.Addr().(*net.TCPAddr).Port)

	var creds *oauth.OAuthCredentials
	if clientID != "" {
		creds = &oauth.OAuthCredentials{ClientID: clientID, ClientSecret: clientSecret, TokenURL: metadata.TokenEndpoint, Metadata: metadata}
	} else if !metadata.SupportsRegistration() {
		return fmt.Errorf("authorization server does not support dynamic client registration; supply --client-id")
	} else {
		creds, err = flow.Register(ctx, metadata, clientName, redirectURI)
		if err != nil {
			return fmt.Errorf("register client: %w", err)
		}
	}

	_, err = flow.AuthorizeWithBrowser(ctx, creds, listener, func(authURL string) error {
		fmt.Printf("Opening browser for authorization...\n%s\n", authURL)
		return exec.Command("open", authURL).Start()
	})
	return err
}

// buildCreds returns credentials for the OOB flow: uses provided clientID/secret
// if given, otherwise performs dynamic client registration.
func buildCreds(ctx context.Context, flow interface {
	Register(context.Context, *oauth.OAuthMetadata, string, ...string) (*oauth.OAuthCredentials, error)
}, metadata *oauth.OAuthMetadata, clientID, clientSecret, clientName string) (*oauth.OAuthCredentials, error) {
	if clientID != "" {
		return &oauth.OAuthCredentials{ClientID: clientID, ClientSecret: clientSecret, TokenURL: metadata.TokenEndpoint, Metadata: metadata}, nil
	}
	if !metadata.SupportsRegistration() {
		return nil, fmt.Errorf("authorization server does not support dynamic client registration; supply --client-id")
	}
	creds, err := flow.Register(ctx, metadata, clientName)
	if err != nil {
		return nil, fmt.Errorf("register client: %w", err)
	}
	return creds, nil
}
