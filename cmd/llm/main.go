package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"path/filepath"

	// Packages
	kong "github.com/alecthomas/kong"
	client "github.com/mutablelogic/go-client"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	anthropic "github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	session "github.com/mutablelogic/go-llm/pkg/session"
	version "github.com/mutablelogic/go-llm/pkg/version"
	logger "github.com/mutablelogic/go-server/pkg/logger"
	trace "go.opentelemetry.io/otel/trace"
	terminal "golang.org/x/term"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Globals struct {
	// Debug option
	Debug   bool             `name:"debug" help:"Enable debug logging"`
	Version kong.VersionFlag `name:"version" help:"Print version and exit"`

	// API Keys
	GeminiAPIKey    string `name:"gemini-api-key" env:"GEMINI_API_KEY" help:"Google Gemini API key"`
	AnthropicAPIKey string `name:"anthropic-api-key" env:"ANTHROPIC_API_KEY" help:"Anthropic API key"`
	MistralAPIKey   string `name:"mistral-api-key" env:"MISTRAL_API_KEY" help:"Mistral API key"`
	NewsAPIKey      string `name:"news-api-key" env:"NEWS_API_KEY" help:"NewsAPI key"`
	WeatherAPIKey   string `name:"weather-api-key" env:"WEATHER_API_KEY" help:"WeatherAPI key"`
	HAEndpoint      string `name:"ha-endpoint" env:"HA_ENDPOINT" help:"Home Assistant endpoint URL"`
	HAToken         string `name:"ha-token" env:"HA_TOKEN" help:"Home Assistant long-lived access token"`

	// Tool options
	FsDir string `name:"fs" env:"FS_DIR" help:"Root directory for filesystem tools" type:"existingdir"`

	// Open Telemetry options
	OTel struct {
		Endpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" help:"OpenTelemetry endpoint" default:""`
		Header   string `env:"OTEL_EXPORTER_OTLP_HEADERS" help:"OpenTelemetry collector headers"`
		Name     string `env:"OTEL_SERVICE_NAME" help:"OpenTelemetry service name" default:"${EXECUTABLE_NAME}"`
	} `embed:"" prefix:"otel."`

	// Private fields
	ctx    context.Context
	cancel context.CancelFunc
	tracer trace.Tracer
	log    *logger.Logger
	store  session.Store
}

type CLI struct {
	Globals
	AgentCommands
	ModelCommands
	ToolCommands
	EmbeddingCommands
	MessageCommands
	SessionCommands
	MCPCommands
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func main() {
	// Get executable name
	execName := "llm"
	if exe, err := os.Executable(); err == nil {
		execName = filepath.Base(exe)
	}

	// Parse command-line arguments
	cli := new(CLI)
	ctx := kong.Parse(cli,
		kong.Name(execName),
		kong.Description("LLM command line interface"),
		kong.Vars{
			"version":         VersionString(),
			"EXECUTABLE_NAME": execName,
		},
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
	)

	// Run the command
	os.Exit(run(ctx, &cli.Globals))
}

func run(ctx *kong.Context, globals *Globals) int {
	parent := context.Background()

	// Create Logger - use terminal format if stderr is a terminal, otherwise JSON
	if terminal.IsTerminal(int(os.Stderr.Fd())) {
		globals.log = logger.New(os.Stderr, logger.Term, globals.Debug)
	} else {
		globals.log = logger.New(os.Stderr, logger.JSON, globals.Debug)
	}

	// Create the context and cancel function
	globals.ctx, globals.cancel = signal.NotifyContext(parent, os.Interrupt)
	defer globals.cancel()

	// Open Telemetry
	if globals.OTel.Endpoint != "" {
		provider, err := otel.NewProvider(globals.OTel.Endpoint, globals.OTel.Header, globals.OTel.Name)
		if err != nil {
			globals.log.Print(globals.ctx, "OTEL error:", err)
			return -2
		}
		defer provider.Shutdown(context.Background())

		// Store tracer for creating spans
		globals.tracer = provider.Tracer(globals.OTel.Name)
	}

	// Call the Run() method of the selected parsed command.
	if err := ctx.Run(globals); err != nil {
		globals.log.Print(globals.ctx, err)
		return -1
	}

	return 0
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Store returns the session store, creating it lazily.
// Sessions are stored in the user's cache directory.
func (g *Globals) Store() (session.Store, error) {
	if g.store == nil {
		cache, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine cache directory: %w", err)
		}
		dir := filepath.Join(cache, "go-llm", "sessions")
		store, err := session.NewFileStore(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to create session store: %w", err)
		}
		g.store = store
	}
	return g.store, nil
}

// Agent returns an agent with all configured LLM clients
func (g *Globals) Agent() (agent.Agent, error) {
	var opts []agent.Opt

	// Add Google Gemini client if API key is set
	if g.GeminiAPIKey != "" {
		var clientOpts []client.ClientOpt
		if g.Debug {
			clientOpts = append(clientOpts, client.OptTrace(os.Stderr, true))
		}
		geminiClient, err := google.New(g.GeminiAPIKey, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}
		opts = append(opts, agent.WithClient(geminiClient))
	}

	// Add Anthropic client if API key is set
	if g.AnthropicAPIKey != "" {
		var clientOpts []client.ClientOpt
		if g.Debug {
			clientOpts = append(clientOpts, client.OptTrace(os.Stderr, true))
		}
		anthropicClient, err := anthropic.New(g.AnthropicAPIKey, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Anthropic client: %w", err)
		}
		opts = append(opts, agent.WithClient(anthropicClient))
	}

	// Add Mistral client if API key is set
	if g.MistralAPIKey != "" {
		var clientOpts []client.ClientOpt
		if g.Debug {
			clientOpts = append(clientOpts, client.OptTrace(os.Stderr, true))
		}
		mistralClient, err := mistral.New(g.MistralAPIKey, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Mistral client: %w", err)
		}
		opts = append(opts, agent.WithClient(mistralClient))
	}

	// Check if at least one client is configured
	if len(opts) == 0 {
		return nil, fmt.Errorf("no API keys configured. Set --gemini-api-key, --anthropic-api-key, or --mistral-api-key (or use environment variables)")
	}

	return agent.NewAgent(opts...)
}

// VersionString returns the version as a string
func VersionString() string {
	return fmt.Sprintf("%s (%s)", version.GitTag, version.GitSource)
}

///////////////////////////////////////////////////////////////////////////////
// DEBUG TRANSPORT

// debugTransport is an http.RoundTripper that logs requests and responses to stderr
type debugTransport struct {
	base http.RoundTripper
	log  *logger.Logger
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Dump request
	if dump, err := httputil.DumpRequestOut(req, true); err == nil {
		fmt.Fprintf(os.Stderr, "\n>>> REQUEST\n%s\n", dump)
	}

	// Execute request
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// Dump response
	if dump, err := httputil.DumpResponse(resp, true); err == nil {
		fmt.Fprintf(os.Stderr, "\n<<< RESPONSE\n%s\n", dump)
	}

	return resp, nil
}
