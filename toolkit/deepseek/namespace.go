package deepseek

import (
	"slices"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/deepseek/httpclient"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// namespace is the namespace of the DeepSeek web search and fetch tools
type namespace struct {
	tools []llm.Tool
}

var _ toolkit.Namespace = (*namespace)(nil)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the DeepSeek tools
const Name = "deepseek"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace of the DeepSeek web search and fetch tools,
// which share a client with the API key, which is the same key as
// DeepSeek's models'
func New(apiKey string, opts ...client.ClientOpt) (toolkit.Namespace, error) {
	client, err := httpclient.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return &namespace{
		tools: []llm.Tool{
			&webSearch{client: client},
			&webFetch{client: client},
		},
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (namespace) Name() string {
	return Name
}

// Description returns what the namespace's tools are for
func (namespace) Description() string {
	return "Search the web, and fetch web pages, with DeepSeek's web search"
}

// SystemPrompt returns no system prompt, as the tools' descriptions say how
// to use them
func (namespace) SystemPrompt() string {
	return ""
}

// Type returns that the namespace's tools are built in
func (namespace) Type() string { return toolschema.NamespaceBuiltin }

// Notify does nothing, as the namespace's tools don't change
func (namespace) Notify(func(eventschema.EventAction, string)) {}

func (n namespace) Tools() []llm.Tool {
	return slices.Clone(n.tools)
}
