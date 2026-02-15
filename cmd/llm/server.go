package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	// Packages
	"github.com/mutablelogic/go-client"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	"github.com/mutablelogic/go-llm/pkg/homeassistant"
	httphandler "github.com/mutablelogic/go-llm/pkg/httphandler"
	"github.com/mutablelogic/go-llm/pkg/newsapi"
	"github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	"github.com/mutablelogic/go-llm/pkg/provider/google"
	"github.com/mutablelogic/go-llm/pkg/provider/mistral"
	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/session"
	"github.com/mutablelogic/go-llm/pkg/tool"
	version "github.com/mutablelogic/go-llm/pkg/version"
	"github.com/mutablelogic/go-llm/pkg/weatherapi"
	server "github.com/mutablelogic/go-server"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	httpserver "github.com/mutablelogic/go-server/pkg/httpserver"
)

type ServerCommands struct {
	// Commands
	RunServer RunServer `cmd:"" name:"run" help:"Run server." group:"SERVER"`
}

type RunServer struct {
	// Provider API Keys
	GeminiAPIKey    string `name:"gemini-api-key" env:"GEMINI_API_KEY" help:"Google Gemini API key"`
	AnthropicAPIKey string `name:"anthropic-api-key" env:"ANTHROPIC_API_KEY" help:"Anthropic API key"`
	MistralAPIKey   string `name:"mistral-api-key" env:"MISTRAL_API_KEY" help:"Mistral API key"`

	// Tool API Keys
	NewsAPIKey    string `name:"news-api-key" env:"NEWS_API_KEY" help:"NewsAPI key"`
	WeatherAPIKey string `name:"weather-api-key" env:"WEATHER_API_KEY" help:"WeatherAPI key"`
	HAEndpoint    string `name:"ha-endpoint" env:"HA_ENDPOINT" help:"Home Assistant endpoint URL"`
	HAToken       string `name:"ha-token" env:"HA_TOKEN" help:"Home Assistant long-lived access token"`

	// TLS server options
	TLS struct {
		ServerName string `name:"name" help:"TLS server name"`
		CertFile   string `name:"cert" help:"TLS certificate file"`
		KeyFile    string `name:"key" help:"TLS key file"`
	} `embed:"" prefix:"tls."`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *RunServer) Run(ctx *Globals) error {
	return cmd.WithManager(ctx, func(manager *agent.Manager, v string) error {
		// Start the HTTP server and wait for shutdown
		return cmd.Serve(ctx, manager, version.Version())
	})
}

// WithManager creates the resource manager, registers all resource instances
// (logger, otel, handlers, router) in dependency order, invokes fn, then
// closes the manager regardless of whether fn returned an error.
func (cmd *RunServer) WithManager(ctx *Globals, fn func(*agent.Manager, string) error) error {
	// Make client opts
	clientOpts := []client.ClientOpt{}
	if ctx.Debug {
		clientOpts = append(clientOpts, client.OptTrace(os.Stderr, ctx.Verbose))
	}
	if ctx.tracer != nil {
		clientOpts = append(clientOpts, client.OptTracer(ctx.tracer))
	}
	if ctx.HTTP.Timeout != 0 {
		clientOpts = append(clientOpts, client.OptTimeout(ctx.HTTP.Timeout))
	}

	// Anthropic client
	opts := []agent.Opt{}
	if cmd.AnthropicAPIKey != "" {
		anthropicClient, err := anthropic.New(cmd.AnthropicAPIKey, clientOpts...)
		if err != nil {
			return fmt.Errorf("failed to create Anthropic client: %w", err)
		}
		opts = append(opts, agent.WithClient(anthropicClient))
	}

	// Google client
	if cmd.GeminiAPIKey != "" {
		googleClient, err := google.New(cmd.GeminiAPIKey, clientOpts...)
		if err != nil {
			return fmt.Errorf("failed to create Google client: %w", err)
		}
		opts = append(opts, agent.WithClient(googleClient))
	}

	// Mistral client
	if cmd.MistralAPIKey != "" {
		mistralClient, err := mistral.New(cmd.MistralAPIKey, clientOpts...)
		if err != nil {
			return fmt.Errorf("failed to create Mistral client: %w", err)
		}
		opts = append(opts, agent.WithClient(mistralClient))
	}

	// Check if at least one client is configured
	if len(opts) == 0 {
		return fmt.Errorf("no API keys configured. Set --gemini-api-key, --anthropic-api-key, or --mistral-api-key (or use environment variables)")
	}

	// Add a session store
	if store, err := cmd.SessionStore(ctx.execName); err != nil {
		return err
	} else {
		opts = append(opts, agent.WithSessionStore(store))
	}

	// Add new toolkit with news, weather and home assistant tools if API keys are provided
	toolkit, err := tool.NewToolkit()
	if err != nil {
		return fmt.Errorf("failed to create toolkit: %w", err)
	} else {
		opts = append(opts, agent.WithToolkit(toolkit))
	}

	// NewsAPI tool
	if cmd.NewsAPIKey != "" {
		if tool, err := newsapi.NewTools(cmd.NewsAPIKey, clientOpts...); err != nil {
			return fmt.Errorf("failed to create NewsAPI tool: %w", err)
		} else if err := toolkit.Register(tool...); err != nil {
			return fmt.Errorf("failed to add NewsAPI tool to toolkit: %w", err)
		}
	}

	// WeatherAPI tool
	if cmd.WeatherAPIKey != "" {
		if tool, err := weatherapi.NewTools(cmd.WeatherAPIKey, clientOpts...); err != nil {
			return fmt.Errorf("failed to create WeatherAPI tool: %w", err)
		} else if err := toolkit.Register(tool...); err != nil {
			return fmt.Errorf("failed to add WeatherAPI tool to toolkit: %w", err)
		}
	}

	// Home Assistant tool
	if cmd.HAEndpoint != "" && cmd.HAToken != "" {
		if tool, err := homeassistant.NewTools(cmd.HAEndpoint, cmd.HAToken, clientOpts...); err != nil {
			return fmt.Errorf("failed to create Home Assistant tool: %w", err)
		} else if err := toolkit.Register(tool...); err != nil {
			return fmt.Errorf("failed to add Home Assistant tool to toolkit: %w", err)
		}
	}

	// Create the manager
	manager, err := agent.NewManager(opts...)
	if err != nil {
		return err
	}
	defer manager.Close()

	// Run the server with the manager
	return fn(manager, version.Version())
}

// Serve creates the httpserver instance, logs the startup banner, and
// blocks until context cancellation (e.g. SIGINT). The caller is
// responsible for closing the manager afterwards.
func (cmd *RunServer) Serve(ctx *Globals, manager *agent.Manager, versionTag string) error {
	// Create middleware
	middleware := []httprouter.HTTPMiddlewareFunc{}
	if mw, ok := ctx.logger.(server.HTTPMiddleware); ok {
		middleware = append(middleware, mw.WrapFunc)
	}

	// Create the TLS config if TLS options are provided
	var tlsConfig *tls.Config
	if cmd.TLS.CertFile != "" || cmd.TLS.KeyFile != "" {
		var pemData [][]byte
		if cmd.TLS.CertFile != "" {
			certData, err := os.ReadFile(cmd.TLS.CertFile)
			if err != nil {
				return fmt.Errorf("failed to read TLS certificate: %w", err)
			}
			pemData = append(pemData, certData)
		}
		if cmd.TLS.KeyFile != "" {
			keyData, err := os.ReadFile(cmd.TLS.KeyFile)
			if err != nil {
				return fmt.Errorf("failed to read TLS key: %w", err)
			}
			pemData = append(pemData, keyData)
		}
		var err error
		tlsConfig, err = httpserver.TLSConfig(cmd.TLS.ServerName, false, pemData...)
		if err != nil {
			return fmt.Errorf("failed to create TLS config: %w", err)
		}
	}

	// Create the HTTP router
	router, err := httprouter.NewRouter(ctx.ctx, ctx.HTTP.Prefix, ctx.HTTP.Origin, "LLM Server", versionTag, middleware...)
	if err != nil {
		return err
	} else if err := httphandler.RegisterHandlers(manager, router, true); err != nil {
		return err
	}

	// Create the server
	httpserver, err := httpserver.New(ctx.HTTP.Addr, router, tlsConfig)
	if err != nil {
		return err
	}

	// Run the server
	ctx.logger.Printf(ctx.ctx, "%s@%s started on %s", ctx.execName, versionTag, ctx.HTTP.Addr)
	if err := httpserver.Run(ctx.ctx); err != nil {
		return err
	}

	// Return success
	ctx.logger.Printf(ctx.ctx, "%s@%s stopped", ctx.execName, versionTag)
	return nil
}

// SessionStore returns the session store, creating it lazily.
// Sessions are stored in the user's cache directory.
func (cmd *RunServer) SessionStore(execName string) (schema.Store, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}
	store, err := session.NewFileStore(filepath.Join(cache, execName))
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}
	return store, nil
}

/*

// Manager returns an agent manager with all configured LLM clients
func (g *Globals) Manager() (*agent.Manager, error) {
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
*/
