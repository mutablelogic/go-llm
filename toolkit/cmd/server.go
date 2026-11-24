//go:build !client

package cmd

import (
	"errors"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	deepseek "github.com/mutablelogic/go-llm/toolkit/deepseek"
	homeassistant "github.com/mutablelogic/go-llm/toolkit/homeassistant"
	httphandler "github.com/mutablelogic/go-llm/toolkit/httphandler"
	newsapi "github.com/mutablelogic/go-llm/toolkit/newsapi"
	ollama "github.com/mutablelogic/go-llm/toolkit/ollama"
	weatherapi "github.com/mutablelogic/go-llm/toolkit/weatherapi"
	webhook "github.com/mutablelogic/go-llm/toolkit/webhook"
	server "github.com/mutablelogic/go-server"
	servercmd "github.com/mutablelogic/go-server/pkg/cmd"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Flags are the toolkit's flags for the server, which has none yet: the
// tools are added when their credentials are set
type Flags struct{}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// WeatherAPICredential is the key of the credential holding the
	// WeatherAPI key, which adds the weather tools
	WeatherAPICredential = "WEATHERAPI_API_KEY"

	// OllamaCredential is the key of the credential holding the Ollama API
	// key, which adds the web search and fetch tools
	OllamaCredential = "OLLAMA_API_KEY"

	// DeepSeekCredential is the key of the credential holding the DeepSeek
	// API key, which adds the web search and fetch tools, and is the key of
	// DeepSeek's models
	DeepSeekCredential = "DEEPSEEK_API_KEY"

	// NewsAPICredential is the key of the credential holding the NewsAPI key,
	// which adds the news tools
	NewsAPICredential = "NEWSAPI_API_KEY"

	// HomeAssistantCredential is the key of the credential holding the Home
	// Assistant access token, which adds the Home Assistant tools, with the
	// HomeAssistantURLCredential credential
	HomeAssistantCredential = "HOMEASSISTANT_API_KEY"

	// HomeAssistantURLCredential is the key of the credential holding the URL
	// of the Home Assistant server, such as http://homeassistant.local:8123
	HomeAssistantURLCredential = "HOMEASSISTANT_URL"

	// WebhookEndpointCredential is the key of the credential holding the
	// endpoint of the webhook server's API, such as
	// http://localhost:8087/api, which adds the webhook namespace
	WebhookEndpointCredential = "WEBHOOK_ENDPOINT"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithToolkit creates a toolkit for the server, with the server's tracer,
// and the server's client options, such as its timeout, with its name and
// version as the user agent, for the tools' clients, and then the options,
// such as its credentials provider, adds the tools whose credentials are
// set, registers its HTTP handlers with the server, when there is one, and
// then calls the function with the toolkit, returning any error
func (flags Flags) WithToolkit(ctx server.Cmd, srv *servercmd.RunServer, fn func(*toolkit.Toolkit) error, opts ...toolkit.Opt) error {
	_, clientOpts, err := ctx.ClientEndpoint()
	if err != nil {
		return err
	}
	agent := ctx.Name()
	if version := ctx.Version(); version != "" {
		agent += "/" + version
	}
	clientOpts = append(clientOpts, client.OptUserAgent(agent))

	// The server's options come first, so the options are used instead
	opts = append([]toolkit.Opt{
		toolkit.WithTracer(ctx.Tracer()),
		toolkit.WithClientOpts(clientOpts...),
	}, opts...)
	tools, err := toolkit.New(ctx.Context(), opts...)
	if err != nil {
		return err
	}
	defer tools.Close()

	// Add the tools whose credentials are set
	if err := AddTools(ctx, tools); err != nil {
		return err
	}

	// Register the HTTP handlers
	if srv != nil {
		srv.Register(func(router *httprouter.Router) error {
			ctx.Logger().DebugContext(ctx.Context(), "registering llm toolkit http handlers")
			return httphandler.RegisterHandlers(tools, router)
		})
	}

	// Call the function with the toolkit
	return fn(tools)
}

// AddTools adds the tools whose credentials exist to the toolkit, apart from
// the memory, which a system adds, with its store. API keys are read from the
// toolkit's credentials, and the tools' clients are created with the
// toolkit's client options. The tools which are added are logged, and those
// which aren't, such as when their credential doesn't exist, are logged as a
// warning, with why.
func AddTools(ctx server.Cmd, tools *toolkit.Toolkit) error {
	return errors.Join(
		addNamespace(ctx, tools, weatherapi.Name, WeatherAPICredential, weatherapi.New),
		addNamespace(ctx, tools, ollama.Name, OllamaCredential, ollama.New),
		addNamespace(ctx, tools, deepseek.Name, DeepSeekCredential, deepseek.New),
		addNamespace(ctx, tools, newsapi.Name, NewsAPICredential, newsapi.New),
		addHomeAssistantTools(ctx, tools),
	)
}

// AddWebhook adds the webhook namespace to the toolkit, as AddTools adds the
// others, for the webhook server whose endpoint is in the
// WebhookEndpointCredential credential, and returns it, as the server runs
// it, or nil, when it isn't added
func AddWebhook(ctx server.Cmd, tools *toolkit.Toolkit) (*webhook.Namespace, error) {
	var ns *webhook.Namespace
	if err := addNamespace(ctx, tools, webhook.Name, WebhookEndpointCredential, func(endpoint string, opts ...client.ClientOpt) (_ toolkit.Namespace, err error) {
		if ns, err = webhook.New(endpoint, opts...); err != nil {
			return nil, fmt.Errorf("%s: %w", WebhookEndpointCredential, err)
		}
		return ns, nil
	}); err != nil {
		return nil, err
	}
	return ns, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// addHomeAssistantTools adds the Home Assistant tools, for the server whose
// URL is in the HomeAssistantURLCredential credential, when it and the
// access token's credential exist, as AddTools adds the others
func addHomeAssistantTools(ctx server.Cmd, tools *toolkit.Toolkit) error {
	endpoint, ok := readCredential(ctx, tools, homeassistant.Name, HomeAssistantURLCredential)
	if !ok {
		return nil
	}
	return addNamespace(ctx, tools, homeassistant.Name, HomeAssistantCredential, func(apiKey string, opts ...client.ClientOpt) (toolkit.Namespace, error) {
		ns, err := homeassistant.New(endpoint, apiKey, opts...)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", HomeAssistantURLCredential, err)
		}
		return ns, nil
	})
}

// addNamespace adds the namespace of tools returned by fn to the toolkit,
// when the credential holding their API key, or endpoint, can be read, and
// logs its tools. fn is called with it, and the toolkit's client options.
func addNamespace(ctx server.Cmd, tools *toolkit.Toolkit, name, key string, fn func(string, ...client.ClientOpt) (toolkit.Namespace, error)) error {
	value, ok := readCredential(ctx, tools, name, key)
	if !ok {
		return nil
	}
	ns, err := fn(value, tools.ClientOpts()...)
	if err != nil {
		return err
	}
	return add(ctx, tools, ns)
}

// readCredential returns the credential of the namespace, or false when it
// can't be read, such as when it doesn't exist, when a warning is logged,
// with the namespace's name, as its tools aren't added
func readCredential(ctx server.Cmd, tools *toolkit.Toolkit, name, key string) (string, bool) {
	value, err := tools.Credential(ctx.Context(), key)
	if err != nil {
		ctx.Logger().WarnContext(ctx.Context(), "tools not added, as their credential can't be read", "namespace", name, "credential", key, "error", err.Error())
		return "", false
	}
	return value, true
}

// add adds the namespace to the toolkit, and logs its tools
func add(ctx server.Cmd, tools *toolkit.Toolkit, ns toolkit.Namespace) error {
	if err := tools.Add(ns); err != nil {
		return err
	}
	ctx.Logger().InfoContext(ctx.Context(), "tools added", "namespace", ns.Name(), "tools", len(ns.Tools()))
	return nil
}
