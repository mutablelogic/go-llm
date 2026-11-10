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
	ProviderList             ProviderListCmd             `cmd:"" name:"providers" help:"List the providers." group:"LLM PROVIDERS"`
	ProviderGet              ProviderGetCmd              `cmd:"" name:"provider" help:"Get a provider by name." group:"LLM PROVIDERS"`
	ProviderUpdate           ProviderUpdateCmd           `cmd:"" name:"provider-update" help:"Update a provider's URL, patterns or API key." group:"LLM PROVIDERS"`
	ProviderDelete           ProviderDeleteCmd           `cmd:"" name:"provider-delete" help:"Delete a provider by name." group:"LLM PROVIDERS"`
	ProviderCreateOllama     ProviderCreateOllamaCmd     `cmd:"" name:"provider-create-ollama" help:"Create an Ollama provider." group:"LLM PROVIDERS"`
	ProviderCreateLMStudio   ProviderCreateLMStudioCmd   `cmd:"" name:"provider-create-lmstudio" help:"Create an LM Studio provider, for the models on your own computer." group:"LLM PROVIDERS"`
	ProviderCreateEliza      ProviderCreateElizaCmd      `cmd:"" name:"provider-create-eliza" help:"Create an Eliza provider, for testing." group:"LLM PROVIDERS"`
	ProviderCreateGoWhisper  ProviderCreateGoWhisperCmd  `cmd:"" name:"provider-create-gowhisper" help:"Create a go-whisper provider, which transcribes speech with whisper.cpp models." group:"LLM PROVIDERS"`
	ProviderCreateElevenLabs ProviderCreateElevenLabsCmd `cmd:"" name:"provider-create-elevenlabs" help:"Create an ElevenLabs provider, which transcribes speech." group:"LLM PROVIDERS"`
	ProviderCreateOpenAI     ProviderCreateOpenAICmd     `cmd:"" name:"provider-create-openai" help:"Create an OpenAI, or OpenAI-compatible, provider." group:"LLM PROVIDERS"`
	ProviderCreateDeepSeek   ProviderCreateDeepSeekCmd   `cmd:"" name:"provider-create-deepseek" help:"Create a DeepSeek provider." group:"LLM PROVIDERS"`
	ProviderCreateMistral    ProviderCreateMistralCmd    `cmd:"" name:"provider-create-mistral" help:"Create a Mistral provider." group:"LLM PROVIDERS"`
	ProviderCreateAnthropic  ProviderCreateAnthropicCmd  `cmd:"" name:"provider-create-anthropic" help:"Create an Anthropic provider, for Claude's models." group:"LLM PROVIDERS"`
	ProviderCreateGemini     ProviderCreateGeminiCmd     `cmd:"" name:"provider-create-gemini" help:"Create a Gemini provider, for Google's Gemini models." group:"LLM PROVIDERS"`
}

type ProviderListCmd struct {
	schema.ProviderListRequest
}

type ProviderGetCmd struct {
	Name string `arg:"" name:"name" help:"Provider name."`
}

// ProviderUpdateCmd updates a provider, whose URL and patterns are replaced
// with those given, so one which isn't given is cleared, or is the type's
// default, and whose API key is kept, unless one is given
type ProviderUpdateCmd struct {
	ProviderModels
	Name             string `arg:"" name:"name" help:"Provider name."`
	URL              string `name:"url" help:"URL of the provider, or the type's default when it isn't given."`
	APIKeyCredential string `name:"api-key-credential" help:"Name of the credential holding the API key, which replaces it, or it's kept when it isn't given."`
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

// ProviderCreateLMStudioCmd creates a provider for an LM Studio server, whose
// API token is needed only when LM Studio is told to need one
type ProviderCreateLMStudioCmd struct {
	ProviderModels
	Name             string `name:"name" default:"lmstudio" help:"Provider name."`
	URL              string `arg:"" name:"url" optional:"" help:"URL of the LM Studio server, such as localhost:1234, which is the default."`
	APIKeyCredential string `name:"api-key-credential" help:"Name of the credential holding an API token, when LM Studio needs one."`
}

// ProviderCreateGoWhisperCmd creates a provider for a go-whisper server,
// which has no API key
type ProviderCreateGoWhisperCmd struct {
	ProviderModels
	Name string `name:"name" default:"gowhisper" help:"Provider name."`
	URL  string `arg:"" name:"url" help:"URL of the go-whisper server, such as localhost:8081."`
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

// ProviderCreateAnthropicCmd creates a provider for Anthropic's API, at a
// fixed URL, which needs an API key
type ProviderCreateAnthropicCmd struct {
	ProviderModels
	Name             string `name:"name" default:"anthropic" help:"Provider name."`
	APIKeyCredential string `name:"api-key-credential" help:"Name of the credential holding the API key, such as ANTHROPIC_API_KEY." required:""`
}

// ProviderCreateGeminiCmd creates a provider for Google's Gemini API, at a
// fixed URL, which needs an API key
type ProviderCreateGeminiCmd struct {
	ProviderModels
	Name             string `name:"name" default:"gemini" help:"Provider name."`
	APIKeyCredential string `name:"api-key-credential" help:"Name of the credential holding the API key, such as GEMINI_API_KEY." required:""`
}

// ProviderCreateElevenLabsCmd creates a provider for ElevenLabs, at a
// fixed URL, which needs an API key
type ProviderCreateElevenLabsCmd struct {
	ProviderModels
	Name             string `name:"name" default:"elevenlabs" help:"Provider name."`
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
		table := tui.TableFor[*schema.ProviderStatus](tui.SetWidth(width))
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

func (cmd *ProviderUpdateCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.ProviderUpdateCmd", func(ctx context.Context, client *httpclient.Client) error {
		provider, err := client.UpdateProvider(ctx, cmd.Name, schema.ProviderMeta{
			URL:     cmd.URL,
			Include: cmd.Include,
			Exclude: cmd.Exclude,
			APIKey:  placeholder(cmd.APIKeyCredential),
		})
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
	return createProvider(ctx, "llm.ProviderCreateOllamaCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.Ollama,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		URL:     cmd.URL,
		APIKey:  placeholder(cmd.APIKeyCredential),
	})
}

func (cmd *ProviderCreateLMStudioCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateLMStudioCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.LMStudio,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		URL:     cmd.URL,
		APIKey:  placeholder(cmd.APIKeyCredential),
	})
}

// go-whisper has no API key
func (cmd *ProviderCreateGoWhisperCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateGoWhisperCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.GoWhisper,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		URL:     cmd.URL,
	})
}

// Eliza runs in the server, so has no URL or API key
func (cmd *ProviderCreateElizaCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateElizaCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.Eliza,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
	})
}

// OpenAI's own API, without a URL, needs an API key, but other
// OpenAI-compatible servers may not
func (cmd *ProviderCreateOpenAICmd) Run(ctx server.Cmd) error {
	if cmd.URL == "" && strings.TrimSpace(cmd.APIKeyCredential) == "" {
		return fmt.Errorf("an API key is required for OpenAI: use --api-key-credential with the name of the credential holding it")
	}
	return createProvider(ctx, "llm.ProviderCreateOpenAICmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.OpenAI,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		URL:     cmd.URL,
		APIKey:  placeholder(cmd.APIKeyCredential),
	})
}

// DeepSeek has an OpenAI-compatible API, at a fixed URL
func (cmd *ProviderCreateDeepSeekCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateDeepSeekCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.OpenAI,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		URL:     openai.DeepSeekEndpoint,
		APIKey:  placeholder(cmd.APIKeyCredential),
	})
}

// Mistral has an OpenAI-compatible API, at a fixed URL
func (cmd *ProviderCreateMistralCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateMistralCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.OpenAI,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		URL:     openai.MistralEndpoint,
		APIKey:  placeholder(cmd.APIKeyCredential),
	})
}

func (cmd *ProviderCreateAnthropicCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateAnthropicCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.Anthropic,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		APIKey:  placeholder(cmd.APIKeyCredential),
	})
}

func (cmd *ProviderCreateGeminiCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateGeminiCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.Gemini,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		APIKey:  placeholder(cmd.APIKeyCredential),
	})
}

func (cmd *ProviderCreateElevenLabsCmd) Run(ctx server.Cmd) error {
	return createProvider(ctx, "llm.ProviderCreateElevenLabsCmd", schema.Provider{
		Name:    cmd.Name,
		Type:    llm.ElevenLabs,
		Include: cmd.Include,
		Exclude: cmd.Exclude,
		APIKey:  placeholder(cmd.APIKeyCredential),
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// createProvider creates the provider, and prints it
func createProvider(ctx server.Cmd, span string, req schema.Provider) error {
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
