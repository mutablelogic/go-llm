package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	// Packages
	kong "github.com/alecthomas/kong"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Globals struct {
	// Debugging
	Debug   bool `name:"debug" help:"Enable debug output"`
	Verbose bool `name:"verbose" help:"Enable verbose output"`

	// Agents
	Ollama    `embed:"" help:"Ollama configuration"`
	Anthropic `embed:"" help:"Anthropic configuration"`
	Mistral   `embed:"" help:"Mistral configuration"`
	OpenAI    `embed:"" help:"OpenAI configuration"`

	// Tools
	NewsAPI `embed:"" help:"NewsAPI configuration"`

	// Context
	ctx     context.Context
	agent   llm.Agent
	toolkit *tool.ToolKit
	term    *Term
}

type Ollama struct {
	OllamaEndpoint string `env:"OLLAMA_URL" help:"Ollama endpoint"`
}

type Anthropic struct {
	AnthropicKey string `env:"ANTHROPIC_API_KEY" help:"Anthropic API Key"`
}

type Mistral struct {
	MistralKey string `env:"MISTRAL_API_KEY" help:"Mistral API Key"`
}

type OpenAI struct {
	OpenAIKey string `env:"OPENAI_API_KEY" help:"OpenAI API Key"`
}

type NewsAPI struct {
	NewsKey string `env:"NEWSAPI_KEY" help:"News API Key"`
}

type CLI struct {
	Globals

	// Agents, Models and Tools
	Agents ListAgentsCmd `cmd:"" help:"Return a list of agents"`
	Models ListModelsCmd `cmd:"" help:"Return a list of models"`
	Tools  ListToolsCmd  `cmd:"" help:"Return a list of tools"`

	// Commands
	Download DownloadModelCmd `cmd:"" help:"Download a model"`
	Chat     ChatCmd          `cmd:"" help:"Start a chat session"`
}

////////////////////////////////////////////////////////////////////////////////
// MAIN

func main() {
	// Create a cli parser
	cli := CLI{}
	cmd := kong.Parse(&cli,
		kong.Name(execName()),
		kong.Description("LLM agent command line interface"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Vars{},
	)

	// Create a context
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	cli.Globals.ctx = ctx

	// Create a terminal
	term, err := NewTerm(os.Stdout)
	if err != nil {
		cmd.FatalIfErrorf(err)
		return
	} else {
		cli.Globals.term = term
	}

	// Client options
	clientopts := []client.ClientOpt{}
	if cli.Debug || cli.Verbose {
		clientopts = append(clientopts, client.OptTrace(os.Stderr, cli.Verbose))
	}

	// Create an agent
	opts := []llm.Opt{}
	if cli.OllamaEndpoint != "" {
		opts = append(opts, agent.WithOllama(cli.OllamaEndpoint, clientopts...))
	}
	if cli.AnthropicKey != "" {
		opts = append(opts, agent.WithAnthropic(cli.AnthropicKey, clientopts...))
	}
	if cli.MistralKey != "" {
		opts = append(opts, agent.WithMistral(cli.MistralKey, clientopts...))
	}
	if cli.OpenAIKey != "" {
		opts = append(opts, agent.WithOpenAI(cli.OpenAIKey, clientopts...))
	}

	// Make a toolkit
	toolkit := tool.NewToolKit()
	cli.Globals.toolkit = toolkit

	// Register NewsAPI
	if cli.NewsKey != "" {
		if client, err := newsapi.New(cli.NewsKey, clientopts...); err != nil {
			cmd.FatalIfErrorf(err)
		} else if err := client.RegisterWithToolKit(toolkit); err != nil {
			cmd.FatalIfErrorf(err)
		}
	}

	// Append the toolkit
	opts = append(opts, llm.WithToolKit(toolkit))

	// Create the agent
	agent, err := agent.New(opts...)
	cmd.FatalIfErrorf(err)
	cli.Globals.agent = agent

	// Run the command
	if err := cmd.Run(&cli.Globals); err != nil {
		cmd.FatalIfErrorf(err)
		return
	}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func execName() string {
	// The name of the executable
	name, err := os.Executable()
	if err != nil {
		panic(err)
	} else {
		return filepath.Base(name)
	}
}

func clientOpts(cli *CLI) []client.ClientOpt {
	result := []client.ClientOpt{}
	if cli.Debug {
		result = append(result, client.OptTrace(os.Stderr, cli.Verbose))
	}
	return result
}
