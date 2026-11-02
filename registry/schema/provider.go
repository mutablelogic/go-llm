// Package schema has the requests and responses of the registry's API
package schema

import (
	"net/url"
	"slices"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CreateProviderRequest is a request to create a provider, of a type, with
// a name in the registry, so there can be more than one provider of a type,
// and its endpoint and API key, which aren't needed for every type
type CreateProviderRequest struct {
	Name     string `json:"name,omitempty" help:"Provider name in the registry, which is an identifier, starting with a letter, followed by letters, numbers, underscores or hyphens, or its type when omitted" example:"my-ollama" optional:""`
	Provider string `json:"provider" help:"Provider type" enum:"eliza,ollama,openai" example:"ollama"`
	URL      string `json:"url,omitempty" help:"Provider endpoint, which is the type's default when omitted" example:"http://localhost:11434" optional:""`
	APIKey   string `json:"api_key,omitempty" writeonly:"" help:"API key, or a credential's placeholder, such as ${OPENAI_KEY}, which is sent as a bearer token, and isn't returned" optional:""`
}

// ProviderListRequest filters a list of providers
type ProviderListRequest struct {
	Provider string `json:"provider,omitempty" help:"Returns only the providers of this type" example:"ollama" optional:""`
}

// ProviderCapabilities are what a provider can do
type ProviderCapabilities struct {
	Generator   bool `json:"generator,omitempty" readonly:"" help:"Whether the provider generates messages"`
	Embedder    bool `json:"embedder,omitempty" readonly:"" help:"Whether the provider embeds messages"`
	Downloader  bool `json:"downloader,omitempty" readonly:"" help:"Whether the provider downloads and deletes models, such as Ollama"`
	Transcriber bool `json:"transcriber,omitempty" readonly:"" help:"Whether the provider transcribes and translates audio"`
	Recognizer  bool `json:"recognizer,omitempty" readonly:"" help:"Whether the provider recognizes text in images and documents"`
}

// Provider is a provider in the registry: its name, its type, its endpoint,
// what it can do, and the error of its ping, when it can't be reached. Its
// API key isn't returned.
type Provider struct {
	Name     string `json:"name" readonly:"" help:"Provider name in the registry" example:"my-ollama"`
	Provider string `json:"provider" readonly:"" help:"Provider type" example:"ollama"`
	URL      string `json:"url,omitempty" readonly:"" help:"Provider endpoint, without its password" example:"http://localhost:11434"`
	ProviderCapabilities
	Error string `json:"error,omitempty" readonly:"" help:"Why the provider can't be reached, when it can't"`
}

// ProviderList is the providers in the registry, in order of their names
type ProviderList struct {
	ProviderListRequest
	Count uint        `json:"count"`
	Body  []*Provider `json:"body,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// providerTypes are the types of providers which the registry creates
var providerTypes = []string{llm.Eliza, llm.Ollama, llm.OpenAI}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ProviderTypes returns the types of providers which the registry creates,
// in order
func ProviderTypes() []string {
	return slices.Clone(providerTypes)
}

// Query returns the request as the query of a URL
func (r ProviderListRequest) Query() url.Values {
	values := url.Values{}
	if r.Provider != "" {
		values.Set("provider", r.Provider)
	}
	return values
}

// RedactURL returns the URL without its password, or an empty string when
// it's nil
func RedactURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Redacted()
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r CreateProviderRequest) String() string {
	r.APIKey = ""
	return types.Stringify(r)
}

func (r ProviderListRequest) String() string {
	return types.Stringify(r)
}

func (p Provider) String() string {
	return types.Stringify(p)
}

func (l ProviderList) String() string {
	return types.Stringify(l)
}

func (c ProviderCapabilities) String() string {
	var result []string
	for _, capability := range []struct {
		name string
		ok   bool
	}{
		{"generator", c.Generator},
		{"embedder", c.Embedder},
		{"downloader", c.Downloader},
		{"transcriber", c.Transcriber},
		{"recognizer", c.Recognizer},
	} {
		if capability.ok {
			result = append(result, capability.name)
		}
	}
	return strings.Join(result, ", ")
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (p Provider) Header() []string {
	return []string{"Name", "Type", "Status", "Capabilities", "URL"}
}

func (p Provider) Width(col int) int {
	return 0
}

func (p Provider) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return p.Name
	case 1:
		return p.Provider
	case 2:
		// A provider which can't be reached shows why
		if p.Error != "" {
			return p.Error
		}
		return "ok"
	case 3:
		return p.ProviderCapabilities.String()
	case 4:
		return p.URL
	default:
		return ""
	}
}
