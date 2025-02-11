package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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
	Debug   bool          `name:"debug" help:"Enable debug output"`
	Verbose bool          `name:"verbose" short:"v" help:"Enable verbose output"`
	Timeout time.Duration `name:"timeout" help:"Agent connection timeout"`

	// Agents
	Ollama    `embed:"" help:"Ollama configuration"`
	Anthropic `embed:"" help:"Anthropic configuration"`
	Mistral   `embed:"" help:"Mistral configuration"`
	OpenAI    `embed:"" help:"OpenAI configuration"`
	Gemini    `embed:"" help:"Gemini configuration"`

	// Tools
	NewsAPI `embed:"" help:"NewsAPI configuration"`

	// Context
	ctx     context.Context
	agent   *agent.Agent
	toolkit *tool.ToolKit
	term    *Term
	config  *Config
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

type Gemini struct {
	GeminiKey string `env:"GEMINI_API_KEY" help:"Gemini API Key"`
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
	Download  DownloadModelCmd `cmd:"" help:"Download a model (for Ollama)"`
	Chat      ChatCmd          `cmd:"" help:"Start a chat session"`
	Chat2     Chat2Cmd         `cmd:"" help:"Start a chat session (2)"`
	Complete  CompleteCmd      `cmd:"" help:"Complete a prompt, generate image or speech from text"`
	Embedding EmbeddingCmd     `cmd:"" help:"Generate an embedding"`
	Version   VersionCmd       `cmd:"" help:"Print the version of this tool"`
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

	// Load any config
	if config, err := NewConfig(execName()); err != nil {
		cmd.FatalIfErrorf(err)
		return
	} else {
		cli.Globals.config = config
		defer config.Save()
	}

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
	if cli.Timeout > 0 {
		clientopts = append(clientopts, client.OptTimeout(cli.Timeout))
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
	if cli.GeminiKey != "" {
		opts = append(opts, agent.WithGemini(cli.GeminiKey, clientopts...))
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

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func run(globals *Globals, typ Type, name string, fn func(ctx context.Context, model llm.Model) error) error {
	// Obtain the model name from the type
	if name == "" {
		name = globals.config.ModelFor(typ)
	}
	if name == "" {
		return llm.ErrBadParameter.With("No model specified, use --model argument to set model")
	}

	// Get the model
	model, err := globals.agent.GetModel(globals.ctx, name)
	if err != nil {
		return err
	} else {
		globals.config.SetModelFor(typ, model.Name())
	}

	// Run the function
	return fn(globals.ctx, model)
}
