package agent

import (
	"bytes"
	"encoding/json"
	"strconv"
	"text/template"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// PrepareResult holds the output of Prepare — everything needed to
// create a session and send the first message.
type PrepareResult struct {
	SessionMeta schema.SessionMeta // Merged GeneratorMeta + agent name + labels
	Text        string             // Rendered user message from template
	Tools       []string           // Tool names the agent is allowed to use
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Prepare validates the input against the agent's input schema, executes
// the agent's Go template to produce the user message, and merges the
// agent's GeneratorMeta with the provided defaults (agent fields win).
// If parentID is non-empty, it is stored in the session labels.
// The agent name and version are stored in session labels for traceability.
func Prepare(agent *schema.Agent, parentID string, defaults schema.GeneratorMeta, input json.RawMessage) (*PrepareResult, error) {
	if agent == nil {
		return nil, schema.ErrBadParameter.With("agent is required")
	}

	// Merge GeneratorMeta: agent fields win, defaults fill in blanks
	meta := mergeGeneratorMeta(agent.GeneratorMeta, defaults)

	// Validate the input against the agent's input schema
	inputData, err := validateInput(agent.Input, input)
	if err != nil {
		return nil, err
	}

	// Execute the Go template with the input data
	text, err := executeTemplate(agent.Name, agent.Template, inputData)
	if err != nil {
		return nil, err
	}

	// Build session labels
	tags := []string{
		"agent:" + agent.Name + "@" + strconv.FormatUint(uint64(agent.Version), 10),
		"agent_id:" + agent.ID,
	}
	if parentID != "" {
		tags = append(tags, "parent:"+parentID)
	}

	return &PrepareResult{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: meta,
			Title:         &agent.Name,
			Tags:          tags,
		},
		Text:  text,
		Tools: agent.Tools,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// validateInput validates the input JSON against the agent's input schema.
// If the agent has no input schema, any valid JSON object is accepted.
// Returns the unmarshalled input data for use as a template context.
func validateInput(inputSchema schema.JSONSchema, input json.RawMessage) (map[string]any, error) {
	// Unmarshal input into a generic map
	var data map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &data); err != nil {
			return nil, schema.ErrBadParameter.Withf("input: invalid JSON: %v", err)
		}
	}

	// If no schema, accept any input (or no input)
	if len(inputSchema) == 0 {
		if data == nil {
			data = make(map[string]any)
		}
		return data, nil
	}

	// Parse and resolve the schema
	s, err := jsonschema.FromJSON(json.RawMessage(inputSchema))
	if err != nil {
		return nil, schema.ErrBadParameter.Withf("input schema: %v", err)
	}

	validatedInput := input
	if len(validatedInput) == 0 {
		validatedInput = json.RawMessage(`{}`)
	}
	if err := s.Validate(validatedInput); err != nil {
		return nil, schema.ErrBadParameter.Withf("input validation: %v", err)
	}

	if data == nil {
		data = make(map[string]any)
	}
	return data, nil
}

// executeTemplate parses and executes the agent's Go template with the
// given data context. Returns the rendered text.
func executeTemplate(name, tmplText string, data map[string]any) (string, error) {
	if tmplText == "" {
		return "", nil
	}

	tmpl, err := template.New(name).Funcs(funcMap()).Parse(tmplText)
	if err != nil {
		return "", schema.ErrBadParameter.Withf("template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", schema.ErrBadParameter.Withf("template: %v", err)
	}

	return buf.String(), nil
}

// mergeGeneratorMeta merges two GeneratorMeta values. Fields from the agent
// take precedence; defaults fill in any blank fields.
func mergeGeneratorMeta(agent, defaults schema.GeneratorMeta) schema.GeneratorMeta {
	return schema.MergeGeneratorMeta(agent, defaults)
}
