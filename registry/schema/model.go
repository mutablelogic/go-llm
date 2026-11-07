package schema

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model is a model of a provider in the registry
type Model struct {
	Name             string                     `json:"name" readonly:"" help:"Model name" example:"llama3.2:latest"`
	Provider         string                     `json:"provider" readonly:"" help:"Name of the model's provider in the registry" example:"my-ollama"`
	Canonical        string                     `json:"canonical" readonly:"" help:"Provider and model name, separated by a colon, which refers to the model unambiguously" example:"my-ollama:llama3.2:latest"`
	Description      string                     `json:"description,omitempty" readonly:"" help:"Model description"`
	Created          time.Time                  `json:"created,omitzero" readonly:"" help:"When the model was created"`
	Aliases          []string                   `json:"aliases,omitempty" readonly:"" help:"Other names for the model"`
	Meta             map[string]json.RawMessage `json:"meta,omitempty" readonly:"" help:"Provider-specific metadata"`
	InputTokenLimit  *uint                      `json:"input_token_limit,omitempty" readonly:"" help:"Maximum number of input tokens"`
	OutputTokenLimit *uint                      `json:"output_token_limit,omitempty" readonly:"" help:"Maximum number of output tokens"`
	Capabilities     llm.ModelCap               `json:"capabilities" readonly:"" help:"What the model can do, of what's known"`
}

// ModelListRequest filters a list of models
type ModelListRequest struct {
	Provider   string   `json:"provider,omitempty" help:"Returns only the models of the provider with this name in the registry" example:"my-ollama" optional:""`
	Capability []string `json:"capability,omitempty" name:"capability" help:"Returns only the models which are known to have every one of these capabilities: completion, tools, thinking, vision, audio, video, documents, ocr, transcription, translation, embedding, speech or image" example:"vision" optional:""`
}

// LoadModelRequest is a request to load a model, with a provider which
// loads models, such as Ollama, which downloads it when it isn't stored
type LoadModelRequest struct {
	Provider string `json:"provider,omitempty" help:"Name of the provider in the registry to load the model with, which is needed when more than one provider loads models" example:"my-ollama" optional:""`
	Name     string `json:"name" help:"Name of the model to load" example:"llama3.2:latest"`
}

// ProgressEvent is the data of a progress event, streamed while a model is
// loaded
type ProgressEvent struct {
	Status   string  `json:"status" help:"Status of the download, or of the tool, which is its message"`
	Percent  float64 `json:"percent" help:"Percentage which is complete, from 0 to 100, or zero when it isn't known"`
	Progress float64 `json:"progress,omitempty" help:"How far a tool has got, which increases each time, as MCP's progress notification says"`
	Total    float64 `json:"total,omitempty" help:"How far a tool has to go, when it's known"`
	Message  string  `json:"message,omitempty" help:"What a tool is doing, such as that an agent it runs is replying"`
}

// ErrorEvent is the data of an error event, which ends a stream when it
// fails
type ErrorEvent struct {
	Error string `json:"error" help:"Why the model couldn't be loaded"`
}

// ModelList is the models of the providers in the registry, in order of
// their providers' names, and then their names
type ModelList struct {
	ModelListRequest
	Count uint     `json:"count"`
	Body  []*Model `json:"body,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The events of a stream of a model which is loaded: its progress, and
// then the model, or the error. A tool's progress, as a model replies, is
// a progress event too.
const (
	EventProgress = string(llm.StreamProgress)
	EventResult   = "result"
	EventError    = "error"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewModel returns the model, of the provider with the name in the
// registry, with its canonical name
func NewModel(provider, canonical string, model llm.Model) *Model {
	return &Model{
		Name:             model.Name(),
		Provider:         provider,
		Canonical:        canonical,
		Description:      model.Description(),
		Created:          model.Created(),
		Aliases:          model.Aliases(),
		Meta:             model.Meta(),
		InputTokenLimit:  model.InputTokenLimit(),
		OutputTokenLimit: model.OutputTokenLimit(),
		Capabilities:     model.Capabilities(),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Query returns the request as the query of a URL
func (r ModelListRequest) Query() url.Values {
	values := url.Values{}
	if r.Provider != "" {
		values.Set("provider", r.Provider)
	}
	for _, capability := range r.Capability {
		values.Add("capability", capability)
	}
	return values
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Model) String() string {
	return types.Stringify(m)
}

func (r LoadModelRequest) String() string {
	return types.Stringify(r)
}

func (r ModelListRequest) String() string {
	return types.Stringify(r)
}

func (l ModelList) String() string {
	return types.Stringify(l)
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (m Model) Header() []string {
	return []string{"Name", "Provider", "Description", "Input Tokens", "Output Tokens", "Capabilities", "Aliases"}
}

func (m Model) Width(col int) int {
	return 0
}

func (m Model) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return m.Name
	case 1:
		return m.Provider
	case 2:
		return m.Description
	case 3:
		return tokens(m.InputTokenLimit)
	case 4:
		return tokens(m.OutputTokenLimit)
	case 5:
		return m.Capabilities.String()
	case 6:
		return strings.Join(m.Aliases, ", ")
	default:
		return ""
	}
}

// tokens returns the token limit, or nothing when it isn't known
func tokens(limit *uint) string {
	if limit == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*limit), 10)
}
