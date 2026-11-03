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

// ProviderMeta are the fields of a provider which can be set, and changed:
// its endpoint, which isn't needed for every type, the patterns of the names
// of its models, and its API key, which isn't returned
type ProviderMeta struct {
	URL     string   `json:"url,omitempty" help:"Provider endpoint, which is the type's default when omitted, and is returned without its password" example:"http://localhost:11434" optional:""`
	Include []string `json:"include,omitempty" help:"Regular expressions of the names of the models which are included, or every model when there are none" example:"[\"^gpt-\"]" optional:""`
	Exclude []string `json:"exclude,omitempty" help:"Regular expressions of the names of the models which are excluded" example:"[\"-mini$\"]" optional:""`
	APIKey  string   `json:"api_key,omitempty" writeonly:"" help:"API key, or a credential's placeholder, such as ${OPENAI_KEY}, which is sent as a bearer token, and isn't returned" optional:""`
}

// Provider is a provider's definition: its name in the registry, so there
// can be more than one provider of a type, its type, its fields which can be
// set, and its serial, which changes each time its definition does. It's
// what's needed to create it, and what a store of providers holds.
type Provider struct {
	Name string `json:"name,omitempty" help:"Provider name in the registry, which is an identifier, starting with a letter, followed by letters, numbers, underscores or hyphens, or its type when omitted" example:"my-ollama" optional:""`
	Type string `json:"provider" help:"Provider type" enum:"eliza,ollama,openai" example:"ollama"`
	ProviderMeta
	Serial uint64 `json:"serial,omitempty" readonly:"" help:"Revision of the provider's definition, which changes each time it's changed" example:"1"`
}

// ProviderListRequest filters a list of providers
type ProviderListRequest struct {
	Type string `json:"provider,omitempty" help:"Returns only the providers of this type" example:"ollama" optional:""`
}

// ProviderCapabilities are what a provider can do
type ProviderCapabilities struct {
	Generator   bool `json:"generator,omitempty" readonly:"" help:"Whether the provider generates messages"`
	Embedder    bool `json:"embedder,omitempty" readonly:"" help:"Whether the provider embeds messages"`
	Downloader  bool `json:"downloader,omitempty" readonly:"" help:"Whether the provider downloads and deletes models, such as Ollama"`
	Transcriber bool `json:"transcriber,omitempty" readonly:"" help:"Whether the provider transcribes and translates audio"`
	Recognizer  bool `json:"recognizer,omitempty" readonly:"" help:"Whether the provider recognizes text in images and documents"`
}

// ProviderStatus is a provider in the registry: its definition, without its
// API key, and what's known of it now, which is what it can do, and the error
// of its ping, when it can't be reached
type ProviderStatus struct {
	Provider
	ProviderCapabilities
	Error string `json:"error,omitempty" readonly:"" help:"Why the provider can't be reached, when it can't"`
}

// ProviderList is the providers in the registry, in order of their names
type ProviderList struct {
	ProviderListRequest
	Count uint              `json:"count"`
	Body  []*ProviderStatus `json:"body,omitempty"`
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
	if r.Type != "" {
		values.Set("provider", r.Type)
	}
	return values
}

// Equal returns true when the requests create the same provider: with the
// same name and type, and fields
func (r Provider) Equal(other Provider) bool {
	return r.Name == other.Name && r.Type == other.Type && r.ProviderMeta.Equal(other.ProviderMeta)
}

// Equal returns true when the fields are the same: the endpoint, API key,
// or its placeholder, and patterns
func (m ProviderMeta) Equal(other ProviderMeta) bool {
	return m.URL == other.URL && m.APIKey == other.APIKey &&
		slices.Equal(m.Include, other.Include) && slices.Equal(m.Exclude, other.Exclude)
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

func (r Provider) String() string {
	r.APIKey = ""
	return types.Stringify(r)
}

func (m ProviderMeta) String() string {
	m.APIKey = ""
	return types.Stringify(m)
}

func (r ProviderListRequest) String() string {
	return types.Stringify(r)
}

func (p ProviderStatus) String() string {
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

func (p ProviderStatus) Header() []string {
	return []string{"Name", "Type", "Status", "Capabilities", "URL"}
}

func (p ProviderStatus) Width(col int) int {
	return 0
}

func (p ProviderStatus) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return p.Name
	case 1:
		return p.Type
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
