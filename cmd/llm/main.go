package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	// Packages
	kong "github.com/alecthomas/kong"
	client "github.com/mutablelogic/go-client"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	gemini "github.com/mutablelogic/go-llm/pkg/gemini"
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
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
}

type CLI struct {
	Globals
	ModelCommands
	ToolCommands
	EmbeddingCommands
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

// Client returns an agent with all configured LLM clients
func (g *Globals) Client() (agent.Agent, error) {
	var opts []agent.Opt
	var clientOpts []client.ClientOpt

	if g.Debug {
		clientOpts = append(clientOpts, client.OptTrace(os.Stderr, true))
	}

	// Add Google client if GEMINI_API_KEY is set
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		geminiClient, err := gemini.New(apiKey, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}
		opts = append(opts, agent.WithClient(geminiClient))
	}

	// Add Anthropic client if ANTHROPIC_API_KEY is set
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		anthropicClient, err := anthropic.New(apiKey, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Anthropic client: %w", err)
		}
		opts = append(opts, agent.WithClient(anthropicClient))
	}

	// Add Ollama client if OLLAMA_URL is set
	if url := os.Getenv("OLLAMA_URL"); url != "" {
		ollamaClient, err := ollama.New(url, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Ollama client: %w", err)
		}
		// Ping to verify connectivity
		if _, err := ollamaClient.Ping(g.ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to Ollama at %s: %w", url, err)
		}
		opts = append(opts, agent.WithClient(ollamaClient))
	}

	// Check if at least one client is configured
	if len(opts) == 0 {
		return nil, fmt.Errorf("no API keys configured. Set GEMINI_API_KEY, ANTHROPIC_API_KEY, and/or OLLAMA_URL")
	}

	return agent.NewAgent(opts...)
}

// Toolkit returns a toolkit with all configured tools
func (g *Globals) Toolkit() (*tool.Toolkit, error) {
	var tools []tool.Tool
	var clientOpts []client.ClientOpt

	if g.Debug {
		clientOpts = append(clientOpts, client.OptTrace(os.Stderr, true))
	}

	// Add NewsAPI tools if NEWSAPI_KEY is set
	if apiKey := os.Getenv("NEWSAPI_KEY"); apiKey != "" {
		newsTools, err := newsapi.NewTools(apiKey, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create NewsAPI tools: %w", err)
		}
		tools = append(tools, newsTools...)
	}

	// Return empty toolkit if no tools are configured (this is not an error)
	if len(tools) == 0 {
		return tool.NewToolkit()
	}

	return tool.NewToolkit(tools...)
}

// VersionString returns the version as a string
func VersionString() string {
	return fmt.Sprintf("%s (%s)", version.GitTag, version.GitSource)
}
