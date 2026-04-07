package homeassistant

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/homeassistant/httpclient"
	"github.com/mutablelogic/go-llm/kernel/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// RenderTemplateRequest renders a Jinja2 template.
type RenderTemplateRequest struct {
	Template string `json:"template" jsonschema:"The Jinja2 template string to render (e.g. '{{ states(\"sensor.temperature\") }}')."`
}

type renderTemplate struct {
	tool.DefaultTool
	client *httpclient.Client
}

var _ llm.Tool = (*renderTemplate)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*renderTemplate) Name() string { return "ha_template" }

func (*renderTemplate) Description() string {
	return "Render a Home Assistant Jinja2 template. " +
		"Use this to query complex state expressions, perform calculations, or format data. " +
		"Examples: '{{ states(\"sensor.temperature\") }}', " +
		"'{{ states.light | selectattr(\"state\",\"eq\",\"on\") | list | count }} lights are on', " +
		"'{{ as_timestamp(now()) - as_timestamp(states.sensor.last_motion.last_changed) | int }} seconds since motion'."
}

func (*renderTemplate) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[RenderTemplateRequest]()
}

func (t *renderTemplate) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req RenderTemplateRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.Template == "" {
		return nil, schema.ErrBadParameter.With("template is required")
	}

	result, err := t.client.Template(ctx, req.Template)
	if err != nil {
		return nil, err
	}

	return map[string]string{"result": strings.TrimSpace(result)}, nil
}
