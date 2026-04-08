package homeassistant

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/homeassistant/httpclient"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// GetServicesRequest lists available services for a domain.
type GetServicesRequest struct {
	Domain string `json:"domain" jsonschema:"The domain to list services for (e.g. light, switch, climate)."`
}

type getServices struct {
	tool.Base
	client *httpclient.Client
}

var _ llm.Tool = (*getServices)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*getServices) Name() string { return "ha_get_services" }

func (*getServices) Description() string {
	return "List available services for a Home Assistant domain. " +
		"Returns service names and descriptions so you know what actions can be performed. " +
		"Use this before calling ha_call_service if you are unsure which services are available."
}

func (*getServices) InputSchema() *jsonschema.Schema { return jsonschema.MustFor[GetServicesRequest]() }

func (t *getServices) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req GetServicesRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, schema.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}
	if req.Domain == "" {
		return nil, schema.ErrBadParameter.With("domain is required")
	}

	services, err := t.client.Services(ctx, req.Domain)
	if err != nil {
		return nil, err
	}

	type serviceSummary struct {
		Call        string   `json:"call"`
		Name        string   `json:"name,omitempty"`
		Description string   `json:"description,omitempty"`
		Fields      []string `json:"fields,omitempty"`
	}

	result := make([]serviceSummary, 0, len(services))
	for _, s := range services {
		summary := serviceSummary{
			Call:        s.Call,
			Name:        s.Name,
			Description: s.Description,
		}
		for fieldName := range s.Fields {
			summary.Fields = append(summary.Fields, fieldName)
		}
		result = append(result, summary)
	}

	return result, nil
}
