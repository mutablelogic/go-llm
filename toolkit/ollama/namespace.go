package ollama

import (
	"slices"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/ollama/httpclient"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// namespace is the namespace of the Ollama web search and web fetch tools
type namespace struct {
	tools []llm.Tool
}

var _ toolkit.Namespace = namespace{}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the Ollama tools
const Name = "ollama"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace of the Ollama web search and web fetch tools,
// which share a client with the API key
func New(apiKey string, opts ...client.ClientOpt) (toolkit.Namespace, error) {
	client, err := httpclient.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return namespace{
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

// SystemPrompt returns no system prompt, as the tools' descriptions say how
// to use them
func (namespace) SystemPrompt() string {
	return ""
}

func (n namespace) Tools() []llm.Tool {
	return slices.Clone(n.tools)
}
