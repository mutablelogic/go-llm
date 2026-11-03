package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ProviderCommands struct {
	ProviderList           ProviderListCmd           `cmd:"" name:"providers" help:"List the providers." group:"LLM PROVIDERS"`
	ProviderGet            ProviderGetCmd            `cmd:"" name:"provider" help:"Get a provider by name." group:"LLM PROVIDERS"`
	ProviderDelete         ProviderDeleteCmd         `cmd:"" name:"provider-delete" help:"Delete a provider by name." group:"LLM PROVIDERS"`
	ProviderCreateOllama   ProviderCreateOllamaCmd   `cmd:"" name:"provider-create-ollama" help:"Create an Ollama provider." group:"LLM PROVIDERS"`
	ProviderCreateEliza    ProviderCreateElizaCmd    `cmd:"" name:"provider-create-eliza" help:"Create an Eliza provider, for testing." group:"LLM PROVIDERS"`
	ProviderCreateOpenAI   ProviderCreateOpenAICmd   `cmd:"" name:"provider-create-openai" help:"Create an OpenAI, or OpenAI-compatible, provider." group:"LLM PROVIDERS"`
	ProviderCreateDeepSeek ProviderCreateDeepSeekCmd `cmd:"" name:"provider-create-deepseek" help:"Create a DeepSeek provider." group:"LLM PROVIDERS"`
	ProviderCreateMistral  ProviderCreateMistralCmd  `cmd:"" name:"provider-create-mistral" help:"Create a Mistral provider." group:"LLM PROVIDERS"`
}

type ProviderListCmd struct {
	schema.ProviderListRequest
}

type ProviderGetCmd struct {
	Name string `arg:"" name:"name" help:"Provider name."`
}

type ProviderDeleteCmd struct {
	Name string `arg:"" name:"name" help:"Provider name."`
}

// ProviderModels are the patterns of the names of a provider's models
type ProviderModels struct {
	Include []string `name:"include" help:"Regular expressions of the names of the models which are included, or every model when there are none, such as ^gpt-."`
	Exclude []string `name:"exclude" help:"Regular expressions of the names of the models which are excluded, such as -mini$."`
}

type ProviderCreateOllamaCmd struct {
	ProviderModels
	Name             string `name:"name" default:"ollama" help:"Provider name."`
	URL              string `arg:"" name:"url" help:"URL of the Ollama server, such as localhost:11434."`
	APIKeyCredential string `name:"api-key-credential" help:"Name of the credential holding an API key, which is sent as a bearer token, such as for a server behind an authenticating proxy."`
}

type ProviderCreateElizaCmd struct {
	ProviderModels
	Name string `name:"name" default:"eliza" help:"Provider name."`
}

// ProviderCreateOpenAICmd creates a provider for OpenAI, which is the
// default URL, and needs an API key, or a server with an OpenAI-compatible
// API, which may not
type ProviderCreateOpenAICmd struct {
	ProviderModels
	Name             string `name:"name" default:"openai" help:"Provider name."`
	URL              string `name:"url" help:"URL of an OpenAI-compatible server, such as vLLM or LM Studio, rather than OpenAI's."`
	APIKeyCredential string `name:"api-key-credential" help:"Name of the credential holding the API key. Required without a URL."`
}

// ProviderCreateDeepSeekCmd creates a provider for DeepSeek, which has an
// OpenAI-compatible API, at a fixed URL
type ProviderCreateDeepSeekCmd struct {
	ProviderModels
	Name             string `name:"name" default:"deepseek" help:"Provider name."`
	APIKeyCredential string `name:"api-key-credential" help:"Name of the credential holding the API key." required:""`
}

// ProviderCreateMistralCmd creates a provider for Mistral, which has an
// OpenAI-compatible API, at a fixed URL
type ProviderCreateMistralCmd struct {
	ProviderModels
	Name             string `name:"name" default:"mistral" help:"Provider name."`
	APIKeyCredential string `name:"api-key-credential" help:"Name of the credential holding the API key." required:""`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ProviderListCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "llm.ProviderListCmd", func(ctx context.Context, client *httpclient.Client) error {
		providers, err := client.ListProviders(ctx, cmd.ProviderListRequest)
		if err != nil {
			return err
		}

		// With debugging, the response
		if debug {
			fmt.Println(providers)
			return nil
		}

		// Otherwise a table of the providers, and a summary
		table := tui.TableFor[*schema.Provider](tui.SetWidth(width))
		if _, err := table.Write(os.Stdout, providers.Body...); err != nil {
			return err
		}
		summary := tui.TableSummary("providers", providers.Count, uint(len(providers.Body)), 0, nil)
		_, err = summary.Write(os.Stdout)
		return err
	})
}

func (cmd *ProviderGetCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.ProviderGetCmd", func(ctx context.Context, client *httpclient.Client) error {
		provider, err := client.GetProvider(ctx, cmd.Name)
		if err != nil {
			return err
		}
		fmt.Println(provider)
		return nil
	})
}

func (cmd *ProviderDeleteCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.ProviderDeleteCmd", func(ctx context.Context, client *httpclient.Client) error {
		provider, err := client.DeleteProvider(ctx, cmd.Name)
		if err != nil {
			return err
		}
		fmt.Println(provider)
		return nil
	})
}

func (cmd *ProviderCreateOllamaCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateOllamaCmd", schema.CreateProviderRequest{
		Name:     cmd.Name,
		Provider: llm.Ollama,
		Include:  cmd.Include,
		Exclude:  cmd.Exclude,
		URL:      cmd.URL,
		APIKey:   placeholder(cmd.APIKeyCredential),
	})
}

// Eliza runs in the server, so has no URL or API key
func (cmd *ProviderCreateElizaCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateElizaCmd", schema.CreateProviderRequest{
		Name:     cmd.Name,
		Provider: llm.Eliza,
		Include:  cmd.Include,
		Exclude:  cmd.Exclude,
	})
}

// OpenAI's own API, without a URL, needs an API key, but other
// OpenAI-compatible servers may not
func (cmd *ProviderCreateOpenAICmd) Run(ctx server.Cmd) error {
	if cmd.URL == "" && strings.TrimSpace(cmd.APIKeyCredential) == "" {
		return fmt.Errorf("an API key is required for OpenAI: use --api-key-credential with the name of the credential holding it")
	}
	return createProvider(ctx, "llm.ProviderCreateOpenAICmd", schema.CreateProviderRequest{
		Name:     cmd.Name,
		Provider: llm.OpenAI,
		Include:  cmd.Include,
		Exclude:  cmd.Exclude,
		URL:      cmd.URL,
		APIKey:   placeholder(cmd.APIKeyCredential),
	})
}

// DeepSeek has an OpenAI-compatible API, at a fixed URL
func (cmd *ProviderCreateDeepSeekCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateDeepSeekCmd", schema.CreateProviderRequest{
		Name:     cmd.Name,
		Provider: llm.OpenAI,
		Include:  cmd.Include,
		Exclude:  cmd.Exclude,
		URL:      openai.DeepSeekEndpoint,
		APIKey:   placeholder(cmd.APIKeyCredential),
	})
}

// Mistral has an OpenAI-compatible API, at a fixed URL
func (cmd *ProviderCreateMistralCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateMistralCmd", schema.CreateProviderRequest{
		Name:     cmd.Name,
		Provider: llm.OpenAI,
		Include:  cmd.Include,
		Exclude:  cmd.Exclude,
		URL:      openai.MistralEndpoint,
		APIKey:   placeholder(cmd.APIKeyCredential),
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// createProvider creates the provider, and prints it
func createProvider(ctx server.Cmd, span string, req schema.CreateProviderRequest) error {
	return withClient(ctx, span, func(ctx context.Context, client *httpclient.Client) error {
		provider, err := client.CreateProvider(ctx, req)
		if err != nil {
			return err
		}
		fmt.Println(provider)
		return nil
	})
}

// placeholder returns the placeholder of the credential with the name, such
// as ${OPENAI_KEY}, which the server expands, so the API key isn't on the
// command line, or an empty string when there's no name
func placeholder(name string) string {
	if name = strings.TrimSpace(name); name == "" {
		return ""
	}
	return "${" + name + "}"
}
