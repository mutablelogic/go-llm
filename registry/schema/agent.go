package schema

import (
	"bytes"
	"encoding/json"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// PromptMeta is what MCP calls a prompt: a template of a message to a model,
// by its name, with its title and description, and the schema of its input,
// which the template is executed with
type PromptMeta struct {
	Name        string          `json:"name" help:"Name of the agent, which is an identifier, and refers to it" example:"segment"`
	Title       *string         `json:"title,omitempty" help:"Title of the agent, for people" example:"Segment the transcript of an episode" optional:""`
	Description *string         `json:"description,omitempty" help:"What the agent does, for people, and models which call it as a tool" example:"Segments a transcript into its sections" optional:""`
	Input       json.RawMessage `json:"input,omitempty" help:"JSON schema of the input, which is an object, whose properties the prompt uses" example:"{\"type\":\"object\",\"properties\":{\"transcript\":{\"type\":\"string\"}}}" optional:""`
	Prompt      *string         `json:"prompt,omitempty" help:"Go template of the message to the model, executed with the input" example:"Segment this transcript: {{ .transcript }}" optional:""`
}

// AgentMeta is an agent's definition: its prompt, and how a model is asked
// it, which is its model, system prompt, tools, options, and the schema of
// its reply
type AgentMeta struct {
	PromptMeta
	GeneratorMeta
}

// Agent is an agent as it's stored: its definition, and its serial, which
// its store sets, and changes each time it's stored, so a change is made to
// the agent as it was last seen
type Agent struct {
	Serial   uint64    `json:"serial,omitempty" readonly:"" help:"Serial of the agent, which its store changes each time it's stored" example:"3"`
	Created  time.Time `json:"created,omitzero" readonly:"" help:"When the agent was created"`
	Modified time.Time `json:"modified,omitzero" readonly:"" help:"When the agent was last stored"`
	AgentMeta
}

// AgentListRequest selects agents: from an offset, and at most a limit of
// them
type AgentListRequest struct {
	Offset uint  `json:"offset,omitempty" help:"Skips this many agents" example:"0" optional:""`
	Limit  *uint `json:"limit,omitempty" help:"Returns at most this many agents" example:"10" optional:""`
}

// AgentList is the agents a request selects, and how many there are
type AgentList struct {
	AgentListRequest
	Count uint    `json:"count" help:"How many agents there are"`
	Body  []Agent `json:"body,omitzero" help:"Agents, in order of their names"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (p PromptMeta) String() string {
	return types.Stringify(p)
}

func (a AgentMeta) String() string {
	return types.Stringify(a)
}

func (a Agent) String() string {
	return types.Stringify(a)
}

func (r AgentListRequest) String() string {
	return types.Stringify(r)
}

func (r AgentList) String() string {
	return types.Stringify(r)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// IsZero returns true when the prompt sets nothing, apart from its name, so
// as an update, it changes nothing
func (p PromptMeta) IsZero() bool {
	return p.Title == nil && p.Description == nil && p.Prompt == nil && len(bytes.TrimSpace(p.Input)) == 0
}

// Merge returns the prompt, changed by the update: its title, description
// and prompt, when they're set, which empty clears, and its input, when it's
// set, which null clears. The name isn't changed. What the update doesn't
// set is kept.
func (p PromptMeta) Merge(update PromptMeta) PromptMeta {
	result := p
	merge := func(value, update *string) *string {
		if update == nil {
			return value
		} else if strings.TrimSpace(*update) == "" {
			return nil
		}
		return types.Ptr(*update)
	}
	result.Title = merge(p.Title, update.Title)
	result.Description = merge(p.Description, update.Description)
	result.Prompt = merge(p.Prompt, update.Prompt)
	if input := bytes.TrimSpace(update.Input); len(input) > 0 {
		if string(input) == "null" {
			result.Input = nil
		} else {
			result.Input = slices.Clone(update.Input)
		}
	}
	return result
}

// InputSchema returns the schema of the input, or nil when there's none.
// Returns ErrBadParameter when it isn't a JSON schema of an object.
func (p PromptMeta) InputSchema() (*jsonschema.Schema, error) {
	data := bytes.TrimSpace(p.Input)
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	result, err := jsonschema.FromJSON(json.RawMessage(data))
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid input: %v", err)
	} else if result.Type != "object" {
		return nil, llm.ErrBadParameter.With("invalid input: the schema must be of an object")
	}
	return result, nil
}

// Template returns the prompt's template, or nil when there's none. Returns
// ErrBadParameter when it isn't a Go template.
func (p PromptMeta) Template() (*template.Template, error) {
	if p.Prompt == nil || strings.TrimSpace(*p.Prompt) == "" {
		return nil, nil
	}
	result, err := template.New(p.Name).Option("missingkey=error").Parse(*p.Prompt)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid prompt: %v", err)
	}
	return result, nil
}

// IsZero returns true when the definition sets nothing, apart from its
// name, so as an update, it changes nothing
func (a AgentMeta) IsZero() bool {
	return a.PromptMeta.IsZero() && a.GeneratorMeta.IsZero()
}

// Merge returns the definition, changed by the update, as PromptMeta.Merge
// and GeneratorMeta.Merge do
func (a AgentMeta) Merge(update AgentMeta) AgentMeta {
	return AgentMeta{PromptMeta: a.PromptMeta.Merge(update.PromptMeta), GeneratorMeta: a.GeneratorMeta.Merge(update.GeneratorMeta)}
}

// Query returns the request as the parameters of a URL's query
func (r AgentListRequest) Query() url.Values {
	values := url.Values{}
	if r.Offset > 0 {
		values.Set("offset", strconv.FormatUint(uint64(r.Offset), 10))
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(uint64(*r.Limit), 10))
	}
	return values
}
