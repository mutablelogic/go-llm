//go:build !client

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	// Packages
	goclient "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	homeassistant "github.com/mutablelogic/go-llm/homeassistant/connector"
	httphandler "github.com/mutablelogic/go-llm/kernel/httphandler"
	manager "github.com/mutablelogic/go-llm/kernel/manager"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	mcpclient "github.com/mutablelogic/go-llm/mcp/client"
	newsapi "github.com/mutablelogic/go-llm/newsapi/connector"
	session "github.com/mutablelogic/go-llm/pkg/_store"
	anthropic "github.com/mutablelogic/go-llm/provider/anthropic"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	google "github.com/mutablelogic/go-llm/provider/google"
	mistral "github.com/mutablelogic/go-llm/provider/mistral"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	weatherapi "github.com/mutablelogic/go-llm/weatherapi/connector"
	server "github.com/mutablelogic/go-server"
	gocmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

type ServerCommands struct {
	RunServer RunServer `cmd:"" name:"run" help:"Run server." group:"SERVER"`
}

type RunServer struct {
	gocmd.RunServer

	// Provider API Keys
	GeminiAPIKey    string `name:"gemini-api-key" env:"GEMINI_API_KEY" help:"Google Gemini API key"`
	AnthropicAPIKey string `name:"anthropic-api-key" env:"ANTHROPIC_API_KEY" help:"Anthropic API key"`
	MistralAPIKey   string `name:"mistral-api-key" env:"MISTRAL_API_KEY" help:"Mistral API key"`
	OllamaURL       string `name:"ollama-url" env:"OLLAMA_URL" help:"Ollama endpoint URL (e.g. http://localhost:11434/api)"`
	Eliza           bool   `name:"eliza" help:"Include ELIZA provider (no API key required)"`

	// Tool API Keys
	NewsAPIKey    string `name:"news-api-key" env:"NEWS_API_KEY" help:"NewsAPI key"`
	WeatherAPIKey string `name:"weather-api-key" env:"WEATHER_API_KEY" help:"WeatherAPI key"`
	HAEndpoint    string `name:"ha-endpoint" env:"HA_ENDPOINT" help:"Home Assistant endpoint URL"`
	HAToken       string `name:"ha-token" env:"HA_TOKEN" help:"Home Assistant long-lived access token"`

	// Credential store
	Passphrase string `name:"passphrase" env:"LLM_PASSPHRASE" help:"Passphrase for encrypting stored credentials"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (s *RunServer) Run(ctx server.Cmd) error {
	return s.WithManager(ctx, func(mgr *manager.Manager, v string) error {
		s.RunServer.Register(func(router *httprouter.Router) error {
			return httphandler.RegisterHandlers(mgr, router, true)
		})
		return s.RunServer.Run(ctx)
	})
}

// WithManager creates the resource manager, registers all resource instances
// (logger, otel, handlers, router) in dependency order, invokes fn, then
// closes the manager regardless of whether fn returned an error.
func (cmd *RunServer) WithManager(ctx server.Cmd, fn func(*manager.Manager, string) error) error {
	// Derive client opts (trace, tracer, timeout) from the global HTTP flags.
	_, clientOpts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	}

	opts := []manager.Opt{}
	for _, fn := range []func(...goclient.ClientOpt) ([]manager.Opt, error){
		cmd.AnthropicClient,
		cmd.GeminiClient,
		cmd.MistralClient,
		cmd.OllamaClient,
		cmd.ElizaClient,
	} {
		if o, err := fn(clientOpts...); err != nil {
			return err
		} else {
			opts = append(opts, o...)
		}
	}

	// Check if at least one client is configured
	if len(opts) == 0 {
		return fmt.Errorf("no providers configured. Set --gemini-api-key, --anthropic-api-key, --mistral-api-key, --ollama-url (or use environment variables)")
	}

	// Add a session store
	if store, err := cmd.SessionStore(ctx.Name()); err != nil {
		return err
	} else {
		opts = append(opts, manager.WithSessionStore(store))
	}

	// Add an agent store
	if store, err := cmd.AgentStore(ctx.Name()); err != nil {
		return err
	} else {
		opts = append(opts, manager.WithAgentStore(store))
	}

	// Add a credential store (requires passphrase)
	if cmd.Passphrase != "" {
		if store, err := cmd.CredentialStore(ctx.Name()); err != nil {
			return err
		} else {
			opts = append(opts, manager.WithCredentialStore(store))
		}
	} else {
		ctx.Logger().InfoContext(ctx.Context(), "No --passphrase set; credential store disabled")
	}

	// Add a connector store
	if store, err := cmd.ConnectorStore(ctx.Name()); err != nil {
		return err
	} else {
		opts = append(opts, manager.WithConnectorStore(store))
	}

	// NewsAPI tool
	if cmd.NewsAPIKey != "" {
		if tools, err := newsapi.NewTools(cmd.NewsAPIKey, clientOpts...); err != nil {
			return fmt.Errorf("failed to create NewsAPI tool: %w", err)
		} else {
			opts = append(opts, manager.WithTools(tools...))
		}
	}

	// WeatherAPI tool
	if cmd.WeatherAPIKey != "" {
		if tools, err := weatherapi.NewTools(cmd.WeatherAPIKey, clientOpts...); err != nil {
			return fmt.Errorf("failed to create WeatherAPI tool: %w", err)
		} else {
			opts = append(opts, manager.WithTools(tools...))
		}
	}

	// Home Assistant tool
	if cmd.HAEndpoint != "" && cmd.HAToken != "" {
		if tools, err := homeassistant.NewTools(cmd.HAEndpoint, cmd.HAToken, clientOpts...); err != nil {
			return fmt.Errorf("failed to create Home Assistant tool: %w", err)
		} else {
			opts = append(opts, manager.WithTools(tools...))
		}
	}

	// Add tracer if configured
	if ctx.Tracer() != nil {
		opts = append(opts, manager.WithTracer(ctx.Tracer()))
	}

	// Add the MCP connector factory so CreateConnector probes servers on registration.
	// clientOpts already includes trace, tracer and timeout flags.
	opts = append(opts, manager.WithConnectorFactory(manager.MCPConnectorFactory(ctx.Name(), ctx.Version(),
		mcpclient.WithClientOpt(clientOpts...),
		mcpclient.OptOnResourceUpdated(func(c context.Context, r llm.Resource) {
			ctx.Logger().Info("resource updated", "uri", r.URI(), "name", r.Name())
		}),
	)))
	opts = append(opts, manager.WithLogger(ctx.Logger()))

	// Create the manager
	mgr, err := manager.NewManager(ctx.Name(), ctx.Version(), opts...)
	if err != nil {
		return err
	}
	defer mgr.Close()

	// Run the server with the manager
	return fn(mgr, ctx.Version())
}

// ConnectorStore returns the connector store, creating it lazily.
// Connectors are stored as JSON files in the user's cache directory.
func (cmd *RunServer) ConnectorStore(execName string) (schema.ConnectorStore, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}
	store, err := session.NewFileConnectorStore(filepath.Join(cache, execName, "connectors"))
	if err != nil {
		return nil, fmt.Errorf("failed to create connector store: %w", err)
	}
	return store, nil
}

// SessionStore returns the session store, creating it lazily.
// Sessions are stored in the user's cache directory.
func (cmd *RunServer) SessionStore(execName string) (schema.SessionStore, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}
	store, err := session.NewFileSessionStore(filepath.Join(cache, execName, "sessions"))
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}
	return store, nil
}

// AgentStore returns the agent store, creating it lazily.
// Agents are stored in the user's cache directory.
func (cmd *RunServer) AgentStore(execName string) (schema.AgentStore, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}
	store, err := session.NewFileAgentStore(filepath.Join(cache, execName, "agents"))
	if err != nil {
		return nil, fmt.Errorf("failed to create agent store: %w", err)
	}
	return store, nil
}

// CredentialStore returns the credential store, creating it lazily.
// Credentials are stored encrypted in the user's cache directory.
// The passphrase is validated by the store constructor.
func (cmd *RunServer) CredentialStore(execName string) (schema.CredentialStore, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}
	store, err := session.NewFileCredentialStore(cmd.Passphrase, filepath.Join(cache, execName, "credentials"))
	if err != nil {
		return nil, fmt.Errorf("failed to create credential store: %w", err)
	}
	return store, nil
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER CLIENTS

func (cmd *RunServer) AnthropicClient(opts ...goclient.ClientOpt) ([]manager.Opt, error) {
	if cmd.AnthropicAPIKey == "" {
		return nil, nil
	}
	c, err := anthropic.New(cmd.AnthropicAPIKey, opts...)
	return []manager.Opt{manager.WithClient(c)}, err
}

func (cmd *RunServer) GeminiClient(opts ...goclient.ClientOpt) ([]manager.Opt, error) {
	if cmd.GeminiAPIKey == "" {
		return nil, nil
	}
	c, err := google.New(cmd.GeminiAPIKey, opts...)
	return []manager.Opt{manager.WithClient(c)}, err
}

func (cmd *RunServer) MistralClient(opts ...goclient.ClientOpt) ([]manager.Opt, error) {
	if cmd.MistralAPIKey == "" {
		return nil, nil
	}
	c, err := mistral.New(cmd.MistralAPIKey, opts...)
	return []manager.Opt{manager.WithClient(c)}, err
}

func (cmd *RunServer) OllamaClient(opts ...goclient.ClientOpt) ([]manager.Opt, error) {
	if cmd.OllamaURL == "" {
		return nil, nil
	}
	c, err := ollama.New(cmd.OllamaURL, opts...)
	return []manager.Opt{manager.WithClient(c)}, err
}

func (cmd *RunServer) ElizaClient(opts ...goclient.ClientOpt) ([]manager.Opt, error) {
	if !cmd.Eliza {
		return nil, nil
	}
	c, err := eliza.New()
	return []manager.Opt{manager.WithClient(c)}, err
}
