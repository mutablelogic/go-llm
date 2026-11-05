package memory

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// setDetail is the tool which sets a detail, which is its value, that it's
// declined, or forgets it
type setDetail struct {
	toolkit.Base
	memory *Memory
}

var _ llm.Tool = (*setDetail)(nil)

// setDetailInput is the input of the set_detail tool
type setDetailInput struct {
	Detail   string `json:"detail" enum:"name,assistant,location,timezone,language" help:"The detail: the person's name, the assistant's own name, where the person is, their time zone, or the language they speak"`
	Value    string `json:"value,omitempty" help:"Its value, such as a name, a city, an IANA time zone, such as Europe/Berlin, or a language" optional:""`
	Declined bool   `json:"declined,omitempty" help:"The person doesn't want it known, so it's not remembered, or asked for" optional:""`
}

// setDetailOutput is the detail, as it's remembered
type setDetailOutput struct {
	Detail   schema.Field `json:"detail"`
	Value    string       `json:"value,omitempty"`
	Declined bool         `json:"declined,omitempty"`
	Known    bool         `json:"known"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// labels are what each field is, as the person is told, in what the tool
// logs
var labels = map[schema.Field]string{
	schema.FieldName:      "your name",
	schema.FieldAssistant: "my name",
	schema.FieldLocation:  "where you are",
	schema.FieldTimezone:  "your time zone",
	schema.FieldLanguage:  "your language",
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*setDetail) Name() string {
	return "set_detail"
}

func (*setDetail) Description() string {
	return "Remembers a detail of the person you're talking to, or your own name, which every conversation then knows: its value, or that the person declined to say it, so it isn't asked for. With neither, the detail is forgotten."
}

func (*setDetail) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[setDetailInput]()
}

func (*setDetail) OutputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[setDetailOutput]()
}

// Run sets the detail, saying what it's remembering, and returns the
// detail, as it's remembered
func (t *setDetail) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req setDetailInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, llm.ErrBadParameter.With(err.Error())
	}
	field, err := schema.ParseField(req.Detail)
	if err != nil {
		return nil, err
	}

	// The detail, or nil, to forget it
	var detail *schema.Detail
	if req.Value != "" || req.Declined {
		detail = &schema.Detail{Value: req.Value, Declined: req.Declined}
	}
	details, err := t.memory.SetDetail(ctx, field, detail)
	if err != nil {
		return nil, err
	}

	// Say what's remembered
	result := setDetailOutput{Detail: field}
	switch detail := details.Get(field); {
	case detail == nil:
		toolkit.Logf(ctx, "Forgetting %s", labels[field])
	case detail.Declined:
		toolkit.Logf(ctx, "Not remembering %s, as you'd rather not say", labels[field])
		result.Declined, result.Known = true, true
	default:
		toolkit.Logf(ctx, "Remembering %s: %s", labels[field], detail.Value)
		result.Value, result.Known = detail.Value, true
	}
	return result, nil
}
