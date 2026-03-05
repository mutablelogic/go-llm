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
	"time"

	// Packages
	kong "github.com/alecthomas/kong"
	goclient "github.com/mutablelogic/go-client"
	oauth "github.com/mutablelogic/go-client/pkg/oauth"
	mcpclient "github.com/mutablelogic/go-llm/pkg/mcp/client"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

type Globals struct {
	Debug        bool   `name:"debug" short:"d" help:"Trace HTTP requests and responses to stderr"`
	ClientID     string `name:"client-id" help:"OAuth2 client ID"`
	ClientSecret string `name:"client-secret" help:"OAuth2 client secret"`
	CallbackPort int    `name:"callback-port" help:"Loopback port for OAuth callback (0 = random); ignored with --oob" default:"0"`
	OOB          bool   `name:"oob" help:"Out-of-band OAuth: browser displays the code, paste it into the terminal"`

	// Private fields set during main()
	ctx      context.Context
	cancel   context.CancelFunc
	execName string
}

// Connect creates an MCP client for the given URL, starts the background run
// loop and blocks until the session is established or an error occurs.
func (g *Globals) Connect(url string) (*mcpclient.Client, error) {
	var opts []goclient.ClientOpt
	if g.Debug {
		opts = append(opts, goclient.OptTrace(os.Stderr, true))
	}

	var c *mcpclient.Client
	var err error
	c, err = mcpclient.New(url, g.execName, "0", func(ctx context.Context, discoveryURL string) error {
		return g.authorize(ctx, c, discoveryURL)
	}, opts...)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	runErr := make(chan error, 1)
	go func() { runErr <- c.Run(g.ctx) }()

	// stopRun cancels the run goroutine and waits for it to exit.
	stopRun := func() {
		g.cancel()
		<-runErr
	}

	// Poll until the session is established or Run fails.
	for {
		_, err = c.ListTools(g.ctx)
		if err == nil {
			break
		}
		if !errors.Is(err, mcpclient.ErrNotConnected) {
			stopRun()
			return nil, fmt.Errorf("connect: %w", err)
		}
		select {
		case e := <-runErr:
			return nil, fmt.Errorf("connect: %w", e)
		case <-g.ctx.Done():
			stopRun()
			return nil, g.ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}

	return c, nil
}

///////////////////////////////////////////////////////////////////////////////
// CLI

type CLI struct {
	Globals
	List ListCmd `cmd:"" help:"List tools advertised by the server"`
	Call CallCmd `cmd:"" help:"Invoke a named tool on the server"`
}

///////////////////////////////////////////////////////////////////////////////
// MAIN

func main() {
	var execName string
	if exe, err := os.Executable(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	} else {
		execName = filepath.Base(exe)
	}

	cli := new(CLI)
	kctx := kong.Parse(cli,
		kong.Name(execName),
		kong.Description("MCP client"),
		kong.UsageOnError(),
	)

	cli.Globals.execName = execName
	cli.Globals.ctx, cli.Globals.cancel = signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer cli.Globals.cancel()

	if err := kctx.Run(&cli.Globals); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

///////////////////////////////////////////////////////////////////////////////
// OAUTH

// authorize performs the OAuth 2.0 discovery, optional dynamic registration,
// and authorization flow.
func (g *Globals) authorize(ctx context.Context, c *mcpclient.Client, discoveryURL string) error {
	flow := c.OAuth()

	metadata, err := flow.Discover(ctx, discoveryURL)
	if err != nil {
		return fmt.Errorf("discover authorization server: %w", err)
	}

	if g.OOB {
		creds, err := g.buildCreds(ctx, flow, metadata)
		if err != nil {
			return err
		}
		_, err = flow.AuthorizeWithCode(ctx, creds, func(authURL string) (string, error) {
			_ = openBrowser(authURL)
			fmt.Printf("Open this URL to authorize:\n%s\n\nEnter the verification code: ", authURL)
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			return strings.TrimSpace(scanner.Text()), scanner.Err()
		})
		return err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", g.CallbackPort))
	if err != nil {
		return fmt.Errorf("start callback listener: %w", err)
	}
	defer listener.Close()
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", listener.Addr().(*net.TCPAddr).Port)

	var creds *oauth.OAuthCredentials
	if g.ClientID != "" {
		creds = &oauth.OAuthCredentials{ClientID: g.ClientID, ClientSecret: g.ClientSecret, TokenURL: metadata.TokenEndpoint, Metadata: metadata}
	} else if !metadata.SupportsRegistration() {
		return fmt.Errorf("authorization server does not support dynamic client registration; supply --client-id")
	} else {
		creds, err = flow.Register(ctx, metadata, g.execName, redirectURI)
		if err != nil {
			return fmt.Errorf("register client: %w", err)
		}
	}

	_, err = flow.AuthorizeWithBrowser(ctx, creds, listener, func(authURL string) error {
		fmt.Printf("Opening browser for authorization...\n%s\n", authURL)
		return openBrowser(authURL)
	})
	return err
}

// openBrowser attempts to open the given URL in the system browser.
// It falls back gracefully on unsupported platforms (the URL is already printed
// to stdout by the caller, so the user can copy-paste it).
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func (g *Globals) buildCreds(ctx context.Context, flow interface {
	Register(context.Context, *oauth.OAuthMetadata, string, ...string) (*oauth.OAuthCredentials, error)
}, metadata *oauth.OAuthMetadata) (*oauth.OAuthCredentials, error) {
	if g.ClientID != "" {
		return &oauth.OAuthCredentials{ClientID: g.ClientID, ClientSecret: g.ClientSecret, TokenURL: metadata.TokenEndpoint, Metadata: metadata}, nil
	}
	if !metadata.SupportsRegistration() {
		return nil, fmt.Errorf("authorization server does not support dynamic client registration; supply --client-id")
	}
	creds, err := flow.Register(ctx, metadata, g.execName)
	if err != nil {
		return nil, fmt.Errorf("register client: %w", err)
	}
	return creds, nil
}
