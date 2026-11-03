package toolkit

import (
	"bytes"
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Respond is the tool a model calls to reply with JSON which conforms to a
// schema. The tool's input is an object, as the input of a tool must be,
// whose result is the reply, so the reply can be any JSON. The tool's result
// is the reply, when it conforms, or an error the model can correct.
type Respond struct {
	Base
	input *jsonschema.Schema
}

var _ llm.Tool = (*Respond)(nil)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// RespondName is the name of the tool a model calls to reply with JSON
	RespondName = "respond"

	// RespondPrompt is added to the system prompt when the reply has a schema
	RespondPrompt = "Reply in JSON, by calling the " + RespondName + " tool with your final answer as its result, which must match the tool's schema. Don't reply with text instead."

	// respondID is the identifier of the reply's schema, when it doesn't have
	// one, as it's the result of the tool's input, so its references, such as
	// "#/$defs/City", or "#", resolve against it, rather than the input
	respondID = "urn:llm:respond:result"

	// RespondNudge is the message sent when the model replies with text, when
	// the reply has a schema
	RespondNudge = "Reply by calling the " + RespondName + " tool with your answer as its result, which must match the tool's schema."
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewRespond returns the tool a model calls to reply with JSON which
// conforms to the schema. Returns an error if the schema isn't a JSON schema.
func NewRespond(schema json.RawMessage) (*Respond, error) {
	schema = bytes.TrimSpace(schema)
	if len(schema) == 0 || string(schema) == "null" {
		return nil, llm.ErrBadParameter.With("missing schema")
	} else if _, err := jsonschema.FromJSON(schema); err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid schema: %v", err)
	}

	// The input is an object, with the reply as its result, whose schema has
	// an identifier, so its references resolve against it
	schema, err := withID(schema)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid schema: %v", err)
	}
	data, err := json.Marshal(map[string]any{
		"type":                 "object",
		"properties":           map[string]json.RawMessage{"result": schema},
		"required":             []string{"result"},
		"additionalProperties": false,
	})
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid schema: %v", err)
	}
	input, err := jsonschema.FromJSON(data)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid schema: %v", err)
	}
	return &Respond{input: input}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (t *Respond) Name() string {
	return RespondName
}

func (t *Respond) Description() string {
	return "Reply with your final answer, as the result, which must match the schema"
}

func (t *Respond) InputSchema() *jsonschema.Schema {
	return t.input
}

// Run returns the reply, which is the input's result, as JSON, when the input
// conforms to the schema, or an error which says how it doesn't
func (t *Respond) Run(_ context.Context, input json.RawMessage) (any, error) {
	if len(input) == 0 {
		return nil, llm.ErrBadParameter.With("missing result: call the tool with your answer as its result")
	} else if err := t.input.Validate(input); err != nil {
		return nil, llm.ErrBadParameter.Withf("the result doesn't match the schema: %v", err)
	}
	var reply struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(input, &reply); err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid result: %v", err)
	}
	return reply.Result, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// withID returns the schema with an identifier, when it's an object without
// one, so its references resolve against it when it's part of another schema
func withID(schema json.RawMessage) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(schema, &object); err != nil || object == nil {
		// A schema which isn't an object, such as true, has no references
		return schema, nil
	} else if _, exists := object["$id"]; exists {
		return schema, nil
	}
	object["$id"] = json.RawMessage(`"` + respondID + `"`)
	return json.Marshal(object)
}
