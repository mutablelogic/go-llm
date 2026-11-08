package homeassistant

import (
	"slices"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/homeassistant/httpclient"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// namespace is the namespace of the Home Assistant tools
type namespace struct {
	tools []llm.Tool
}

var _ toolkit.Namespace = namespace{}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the Home Assistant tools
const Name = "homeassistant"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace of the Home Assistant tools, for the server at the endpoint, which share a client with the long-lived access token
func New(endpoint, apiKey string, opts ...client.ClientOpt) (toolkit.Namespace, error) {
	client, err := httpclient.New(endpoint, apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return namespace{
		tools: []llm.Tool{
			newQuery(client),
			newControl(client),
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
// Description returns what the namespace's tools are for
func (namespace) Description() string {
	return "Look at, and control, the devices of a Home Assistant smart home"
}

func (namespace) SystemPrompt() string {
	return ""
}

// Notify does nothing, as the namespace's tools don't change
// Type returns that the namespace's tools are built in
func (namespace) Type() string { return toolschema.NamespaceBuiltin }

func (namespace) Notify(func(eventschema.EventAction, string)) {}

func (n namespace) Tools() []llm.Tool {
	return slices.Clone(n.tools)
}
