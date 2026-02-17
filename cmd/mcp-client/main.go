package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"

	// Packages
	kong "github.com/alecthomas/kong"
	client "github.com/mutablelogic/go-client"
	mcpclient "github.com/mutablelogic/go-llm/pkg/mcp/client"
	mcp "github.com/mutablelogic/go-llm/pkg/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CLI struct {
	Globals

	// Commands
	Ping    PingCommand    `cmd:"" help:"Ping the MCP server"`
	Login   LoginCommand   `cmd:"" help:"Login to an MCP server using OAuth"`
	Tools   ToolsCommand   `cmd:"" help:"List available tools"`
	Do      DoCommand      `cmd:"" help:"Call a tool by name"`
	Prompts PromptsCommand `cmd:"" help:"List available prompts"`
	Prompt  PromptCommand  `cmd:"" help:"Get a prompt by name"`
}

type Globals struct {
	Auth  string `name:"auth" help:"Authentication in the form scheme=token (e.g. bearer=TOKEN)" optional:""`
	Debug bool   `name:"debug" help:"Enable debug output" default:"false"`

	// Private
	ctx    context.Context
	cancel context.CancelFunc
	client *mcpclient.Client
}

type PingCommand struct {
	URL string `arg:"" help:"MCP server URL"`
}

type LoginCommand struct {
	URL  string `arg:"" help:"MCP server URL"`
	Port int    `name:"port" help:"Local port for OAuth callback" default:"0"`
}

type ToolsCommand struct {
	URL string `arg:"" help:"MCP server URL"`
}

type DoCommand struct {
	URL  string   `arg:"" help:"MCP server URL"`
	Name string   `arg:"" help:"Tool name"`
	Args []string `arg:"" help:"Tool arguments as key=value pairs" optional:""`
}

type PromptsCommand struct {
	URL string `arg:"" help:"MCP server URL"`
}

type PromptCommand struct {
	URL  string   `arg:"" help:"MCP server URL"`
	Name string   `arg:"" help:"Prompt name"`
	Args []string `arg:"" help:"Prompt arguments as key=value pairs" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func main() {
	cli := CLI{}
	cmd := kong.Parse(&cli,
		kong.Name("mcp-client"),
		kong.Description("MCP (Model Context Protocol) client"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
	)

	// Create context
	cli.ctx, cli.cancel = signal.NotifyContext(context.Background(), os.Interrupt)
	defer cli.cancel()

	// Run the selected command
	cmd.FatalIfErrorf(cmd.Run(&cli.Globals))
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *LoginCommand) Run(g *Globals) error {
	// Discover OAuth metadata
	fmt.Fprintf(os.Stderr, "Discovering OAuth metadata for %s...\n", cmd.URL)
	meta, err := mcpclient.DiscoverOAuth(g.ctx, cmd.URL)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Issuer: %s\n", meta.Issuer)

	// Check PKCE support
	if !meta.SupportsS256() {
		return fmt.Errorf("server does not support S256 PKCE")
	}

	// Start local callback server to get a redirect URI
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cmd.Port))
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", listener.Addr().(*net.TCPAddr).Port)

	// Dynamic client registration
	var clientID string
	if meta.SupportsRegistration() {
		fmt.Fprintf(os.Stderr, "Registering client...\n")
		reg, err := meta.Register(g.ctx, "mcp-client", []string{redirectURI})
		if err != nil {
			listener.Close()
			return err
		}
		clientID = reg.ClientID
		fmt.Fprintf(os.Stderr, "Client ID: %s\n", clientID)
	} else {
		listener.Close()
		return fmt.Errorf("server does not support dynamic client registration; provide a client_id")
	}

	// Generate PKCE challenge
	pkce, err := mcpclient.NewPKCEChallenge()
	if err != nil {
		listener.Close()
		return err
	}

	// Build authorization URL
	authURL := meta.AuthorizationURL(clientID, redirectURI, pkce)
	fmt.Fprintf(os.Stderr, "\nOpen this URL in your browser to authorize:\n\n")
	fmt.Println(authURL)
	fmt.Fprintf(os.Stderr, "\nWaiting for callback...\n")

	// Wait for the OAuth callback
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			desc := r.URL.Query().Get("error_description")
			http.Error(w, "Authorization failed: "+errMsg, http.StatusBadRequest)
			errCh <- fmt.Errorf("authorization failed: %s: %s", errMsg, desc)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			errCh <- fmt.Errorf("callback missing authorization code")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Authorization successful</h2><p>You can close this window.</p></body></html>")
		codeCh <- code
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for code or error or context cancellation
	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		server.Close()
		return err
	case <-g.ctx.Done():
		server.Close()
		return g.ctx.Err()
	}
	server.Close()

	// Exchange code for token
	fmt.Fprintf(os.Stderr, "Exchanging authorization code for token...\n")
	token, err := meta.ExchangeCode(g.ctx, clientID, code, redirectURI, pkce)
	if err != nil {
		return err
	}

	// Output token as JSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(token)
}

func (cmd *PingCommand) Run(g *Globals) error {
	if err := g.connect(cmd.URL); err != nil {
		return err
	}
	defer g.client.Close()

	if err := g.client.Ping(g.ctx); err != nil {
		return err
	}
	fmt.Println("OK")

	// Print server info
	info := g.client.ServerInfo()
	if info != nil {
		fmt.Printf("Server: %s %s (protocol %s)\n", info.ServerInfo.Name, info.ServerInfo.Version, info.Version)
		fmt.Printf("Capabilities: tools=%v prompts=%v resources=%v logging=%v\n",
			info.Capabilities.Tools != nil,
			info.Capabilities.Prompts != nil,
			info.Capabilities.Resources != nil,
			info.Capabilities.Logging != nil,
		)
	}
	return nil
}

func (cmd *ToolsCommand) Run(g *Globals) error {
	if err := g.connect(cmd.URL); err != nil {
		return err
	}
	defer g.client.Close()

	tools, err := g.client.ListTools(g.ctx)
	if err != nil {
		return err
	}
	for i, tool := range tools {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s\n", tool.Name)
		if tool.Description != "" {
			fmt.Printf("  %s\n", tool.Description)
		}
		if tool.InputSchema != nil {
			data, err := json.MarshalIndent(tool.InputSchema, "  ", "  ")
			if err == nil {
				fmt.Printf("  %s\n", string(data))
			}
		}
	}
	fmt.Printf("\n%d tools\n", len(tools))
	return nil
}

func (cmd *DoCommand) Run(g *Globals) error {
	if err := g.connect(cmd.URL); err != nil {
		return err
	}
	defer g.client.Close()

	// Parse key=value args into JSON object
	args, err := parseArgsJSON(cmd.Args)
	if err != nil {
		return err
	}

	result, err := g.client.CallTool(g.ctx, cmd.Name, args)
	if err != nil {
		return err
	}

	if result.Error {
		fmt.Fprintln(os.Stderr, "Tool returned an error")
	}
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			fmt.Println(c.Text)
		default:
			fmt.Printf("[%s] %s\n", c.Type, c.MimeType)
		}
	}
	return nil
}

func (cmd *PromptsCommand) Run(g *Globals) error {
	if err := g.connect(cmd.URL); err != nil {
		return err
	}
	defer g.client.Close()

	prompts, err := g.client.ListPrompts(g.ctx)
	if err != nil {
		return err
	}
	for _, p := range prompts {
		fmt.Printf("%-30s %s\n", p.Name, p.Description)
		for _, arg := range p.Arguments {
			req := ""
			if arg.Required {
				req = " (required)"
			}
			fmt.Printf("  %-28s %s%s\n", arg.Name, arg.Description, req)
		}
	}
	fmt.Printf("\n%d prompts\n", len(prompts))
	return nil
}

func (cmd *PromptCommand) Run(g *Globals) error {
	if err := g.connect(cmd.URL); err != nil {
		return err
	}
	defer g.client.Close()

	// Parse key=value args into string map
	args := make(map[string]string)
	for _, kv := range cmd.Args {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("argument must be key=value, got %q", kv)
		}
		args[parts[0]] = parts[1]
	}

	result, err := g.client.GetPrompt(g.ctx, cmd.Name, args)
	if err != nil {
		return err
	}
	if result.Description != "" {
		fmt.Println(result.Description)
		fmt.Println()
	}
	for i, msg := range result.Messages {
		fmt.Printf("[%d] %s (%s):\n", i, msg.Role, msg.Content.Type)
		if msg.Content.Text != "" {
			fmt.Println(msg.Content.Text)
		}
		fmt.Println()
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// connect creates and stores the MCP client on Globals
func (g *Globals) connect(url string) error {
	var opts []client.ClientOpt
	var token client.Token
	if g.Auth != "" {
		parts := strings.SplitN(g.Auth, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("--auth must be in the form scheme=token (e.g. bearer=TOKEN)")
		}
		scheme := parts[0]
		if strings.EqualFold(scheme, "bearer") {
			scheme = client.Bearer
		}
		token = client.Token{Scheme: scheme, Value: parts[1]}
		opts = append(opts, client.OptReqToken(token))
	}
	if g.Debug {
		opts = append(opts, client.OptTrace(os.Stderr, true))
	}

	c, err := mcpclient.New(url, mcp.ClientInfo{
		Name:    "mcp-client",
		Version: "0.0.1",
	}, opts...)
	if err != nil {
		return err
	}

	// Store token for SSE transport raw HTTP requests
	if token.Value != "" {
		c.SetToken(token)
	}

	// Set notification callback
	c.OnNotification(func(method string, params json.RawMessage) {
		fmt.Printf("notification: %s %s\n", method, string(params))
	})

	g.client = c
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// parseArgsJSON converts key=value pairs to a JSON object (json.RawMessage).
// Returns nil if no args are provided.
func parseArgsJSON(args []string) (json.RawMessage, error) {
	if len(args) == 0 {
		return nil, nil
	}
	m := make(map[string]any, len(args))
	for _, kv := range args {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("argument must be key=value, got %q", kv)
		}
		// Try to parse value as JSON (for numbers, booleans, objects)
		var v any
		if err := json.Unmarshal([]byte(parts[1]), &v); err != nil {
			// Fall back to string
			v = parts[1]
		}
		m[parts[0]] = v
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
