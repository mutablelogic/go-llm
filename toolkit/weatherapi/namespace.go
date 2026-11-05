package weatherapi

import (
	"slices"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/weatherapi/httpclient"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// namespace is the namespace of the WeatherAPI tools
type namespace struct {
	tools []llm.Tool
}

var _ toolkit.Namespace = namespace{}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the WeatherAPI tools
const Name = "weatherapi"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace of the WeatherAPI tools, which share a client with the API key
func New(apiKey string, opts ...client.ClientOpt) (toolkit.Namespace, error) {
	client, err := httpclient.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return namespace{
		tools: []llm.Tool{
			&currentWeather{client: client},
			&forecastWeather{client: client},
			&alertsWeather{client: client},
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

// Notify does nothing, as the namespace's tools don't change
func (namespace) Notify(func(eventschema.EventAction, string)) {}

func (n namespace) Tools() []llm.Tool {
	return slices.Clone(n.tools)
}
