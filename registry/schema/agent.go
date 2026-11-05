package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
	yaml "go.yaml.in/yaml/v3"
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

// RunAgentRequest runs an agent: its input, which is validated against the
// agent's input schema, and executed with its prompt, and the session it's
// run from, such as the conversation whose model called it, as a tool, whose
// model it's run with, when it has none
type RunAgentRequest struct {
	Input   json.RawMessage `json:"input,omitempty" help:"Input of the agent, which its input schema validates, and its prompt is executed with" example:"{\"transcript\":\"...\"}" optional:""`
	Session uuid.UUID       `json:"session,omitzero" help:"Identifier of the session the agent is run from, whose model it's run with, when it has none" example:"3b241101-e2bb-4255-8caf-4136c566a962" optional:""`
}

// RunAgentResponse is what an agent did: the agent, by its name, and the
// serial of the definition which was run, the messages it added, which are
// its prompt, its reply, and the tools it called, and the tokens used, and
// its reply, which is text, or JSON, when the agent has a schema
type RunAgentResponse struct {
	Name   string `json:"name" help:"Name of the agent" example:"segment"`
	Serial uint64 `json:"serial,omitempty" help:"Serial of the agent's definition which was run" example:"3"`
	SessionResponse
	Reply json.RawMessage `json:"reply,omitempty" help:"The agent's reply, as a JSON string of its text, or JSON, when the agent has a schema"`
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
	Body  []Agent `json:"body,omitzero" help:"Agents, in the store's order"`
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

func (r RunAgentRequest) String() string {
	return types.Stringify(r)
}

func (r RunAgentResponse) String() string {
	return types.Stringify(r)
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
	result, err := template.New(p.Name).Option("missingkey=error").Funcs(promptFuncs).Parse(*p.Prompt)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid prompt: %v", err)
	}
	return result, nil
}

// Validate returns ErrBadParameter when the input isn't an object, which its
// input schema, when it has one, validates. No input is an empty object.
func (p PromptMeta) Validate(input json.RawMessage) error {
	_, err := p.input(input)
	return err
}

// Message returns the message to the model, which is the prompt, executed
// with the input, which is validated, or the input, as JSON, when there's
// no prompt. A field of the input which the prompt uses, and the input
// hasn't, is an error. Returns ErrBadParameter when the input isn't valid,
// the prompt can't be executed with it, or there's neither a prompt, nor
// an input.
func (p PromptMeta) Message(input json.RawMessage) (string, error) {
	value, err := p.input(input)
	if err != nil {
		return "", err
	}
	tmpl, err := p.Template()
	if err != nil {
		return "", err
	} else if tmpl == nil {
		if len(value) == 0 {
			return "", llm.ErrBadParameter.Withf("agent %q has no prompt, nor input", p.Name)
		}
		data, err := json.Marshal(value)
		return string(data), err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, value); err != nil {
		return "", llm.ErrBadParameter.Withf("invalid input: %v", err)
	}
	return b.String(), nil
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

// ParseAgentMeta returns the definition in the data, which is YAML, such as
// go-nanoradio's agent/segment.yaml, or JSON, which is YAML too, by the
// names its JSON has, so its input, options and schema are JSON. A key which
// isn't one of its names, such as system_prompt, rather than system, is an
// error, as is an alias, such as *name. Returns ErrBadParameter when it
// can't be parsed.
func ParseAgentMeta(data []byte) (AgentMeta, error) {
	// The YAML is JSON, in the order of its keys, so the input and schema
	// are as they were written
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return AgentMeta{}, llm.ErrBadParameter.Withf("invalid agent: %v", err)
	} else if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return AgentMeta{}, llm.ErrBadParameter.With("invalid agent: it must be a mapping of its fields")
	}
	var buf bytes.Buffer
	if err := writeJSON(&buf, node.Content[0]); err != nil {
		return AgentMeta{}, llm.ErrBadParameter.Withf("invalid agent: %v", err)
	}
	data = buf.Bytes()
	var meta AgentMeta
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&meta); err != nil {
		return AgentMeta{}, llm.ErrBadParameter.Withf("invalid agent: %v", err)
	}
	return meta, nil
}

// YAML returns the definition as YAML, as a file of an agent's definition
// is, which ParseAgentMeta parses: by the names its JSON has, in the same
// order, in block style, with text of more than one line, such as its
// prompt, as a literal block
func (a AgentMeta) YAML() ([]byte, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}

	// JSON is YAML, which is decoded into nodes, in the order of its keys,
	// whose flow style, as JSON is, is changed to block style
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	blockStyle(&node)
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&node); err != nil {
		return nil, err
	} else if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (a Agent) Header() []string {
	return []string{"Name", "Title", "Model", "Tools", "Updated"}
}

func (a Agent) Width(col int) int {
	return 0
}

// Cell returns the agent's name, title, model, tools, and when it was last
// updated, which is when it was created, or modified, whichever is later
func (a Agent) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return a.Name
	case 1:
		return types.Value(a.Title)
	case 2:
		return types.Value(a.Model)
	case 3:
		return strings.Join(a.Tools, ", ")
	case 4:
		return updated(a.Created, a.Modified)
	}
	return ""
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// updated returns when something was last updated, which is when it was
// created, or modified, whichever is later, in local time, or nothing when
// neither is known
func updated(created, modified time.Time) string {
	when := created
	if modified.After(when) {
		when = modified
	}
	if when.IsZero() {
		return ""
	}
	return when.Local().Format(time.DateTime)
}

// input returns the input, decoded as an object, with its numbers as they
// were written, which its input schema, when it has one, validates. No
// input is an empty object.
func (p PromptMeta) input(input json.RawMessage) (map[string]any, error) {
	data := bytes.TrimSpace(input)
	if len(data) == 0 || string(data) == "null" {
		data = []byte("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid input, which must be an object: %v", err)
	} else if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, llm.ErrBadParameter.With("invalid input, which must be one object, with nothing after it")
	}
	schema, err := p.InputSchema()
	if err != nil {
		return nil, err
	} else if schema != nil {
		if err := schema.Validate(data); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid input: %v", err)
		}
	}
	return value, nil
}

// promptFuncs are the functions which a prompt can use: json, which
// returns a value as JSON, such as {{ json .options }}
var promptFuncs = template.FuncMap{
	"json": func(v any) (string, error) {
		data, err := json.Marshal(v)
		return string(data), err
	},
}

// writeJSON writes the YAML node as JSON, with the keys of its mappings in
// their order, which they're decoded in, unlike a map's. A mapping's keys
// are text.
func writeJSON(buf *bytes.Buffer, node *yaml.Node) error {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			buf.WriteString("null")
			return nil
		}
		return writeJSON(buf, node.Content[0])
	case yaml.AliasNode:
		// An alias isn't followed, as one which refers to itself, or many
		// which refer to others, are expanded without end
		return fmt.Errorf("line %d: aliases aren't allowed", node.Line)
	case yaml.MappingNode:
		buf.WriteByte('{')
		for i := 0; i+1 < len(node.Content); i += 2 {
			if i > 0 {
				buf.WriteByte(',')
			}
			var key string
			if err := node.Content[i].Decode(&key); err != nil {
				return fmt.Errorf("line %d: a key must be text: %w", node.Content[i].Line, err)
			}
			data, err := json.Marshal(key)
			if err != nil {
				return err
			}
			buf.Write(data)
			buf.WriteByte(':')
			if err := writeJSON(buf, node.Content[i+1]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case yaml.SequenceNode:
		buf.WriteByte('[')
		for i, child := range node.Content {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeJSON(buf, child); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	default:
		var value any
		if err := node.Decode(&value); err != nil {
			return fmt.Errorf("line %d: %w", node.Line, err)
		}
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("line %d: %w", node.Line, err)
		}
		buf.Write(data)
	}
	return nil
}

// blockStyle changes the style of the node, and its nodes, to block style,
// with text of more than one line as a literal block, and other text plain,
// unless it must be quoted, which the encoder does
func blockStyle(node *yaml.Node) {
	switch {
	case node.Kind == yaml.ScalarNode && node.Tag == "!!str" && strings.Contains(strings.TrimRight(node.Value, "\n"), "\n"):
		node.Style = yaml.LiteralStyle
	default:
		node.Style = 0
	}
	for _, child := range node.Content {
		blockStyle(child)
	}
}
