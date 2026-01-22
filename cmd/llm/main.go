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
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	google "github.com/mutablelogic/go-llm/pkg/google"
	version "github.com/mutablelogic/go-llm/pkg/version"
	trace "go.opentelemetry.io/otel/trace"
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
}

type CLI struct {
	Globals
	ModelCommands
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

	// Create the context and cancel function
	globals.ctx, globals.cancel = signal.NotifyContext(parent, os.Interrupt)
	defer globals.cancel()

	// Open Telemetry
	if globals.OTel.Endpoint != "" {
		provider, err := otel.NewProvider(globals.OTel.Endpoint, globals.OTel.Header, globals.OTel.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			return -2
		}
		defer provider.Shutdown(context.Background())

		// Store tracer for creating spans
		globals.tracer = provider.Tracer(globals.OTel.Name)
	}

	// Call the Run() method of the selected parsed command.
	if err := ctx.Run(globals); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return -1
	}

	return 0
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Client returns an agent with all configured LLM clients
func (g *Globals) Client() (llm.Client, error) {
	var opts []agent.Opt
	var clientOpts []client.ClientOpt

	if g.Debug {
		clientOpts = append(clientOpts, client.OptTrace(os.Stderr, true))
	}

	// Add Google client if GEMINI_API_KEY is set
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		googleClient, err := google.New(apiKey, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Google client: %w", err)
		}
		opts = append(opts, agent.WithClient(googleClient))
	}

	// Add Anthropic client if ANTHROPIC_API_KEY is set
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		anthropicClient, err := anthropic.New(apiKey, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Anthropic client: %w", err)
		}
		opts = append(opts, agent.WithClient(anthropicClient))
	}

	// Check if at least one client is configured
	if len(opts) == 0 {
		return nil, fmt.Errorf("no API keys configured. Set GEMINI_API_KEY and/or ANTHROPIC_API_KEY")
	}

	return agent.NewAgent(opts...)
}

// VersionString returns the version as a string
func VersionString() string {
	return fmt.Sprintf("%s (%s)", version.GitTag, version.GitSource)
}
