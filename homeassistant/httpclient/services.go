package homeassistant

import (
	"context"
	"maps"
	"net/url"
	"slices"

	// Packages
	"github.com/mutablelogic/go-client"
	haschema "github.com/mutablelogic/go-llm/homeassistant/schema"
	"github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Domain = haschema.Domain
type Service = haschema.Service
type Field = haschema.Field
type Selector = haschema.Selector

type reqCall struct {
	Entity string `json:"entity_id"`
}

type CallResponse = haschema.CallResponse

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// Domains returns all domains and their associated service objects
func (c *Client) Domains(ctx context.Context) ([]*Domain, error) {
	var response []*Domain
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("services")); err != nil {
		return nil, err
	}

	return response, nil
}

// Return callable services for a domain
func (c *Client) Services(ctx context.Context, domain string) ([]*Service, error) {
	var response []Domain
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("services")); err != nil {
		return nil, err
	}
	for _, v := range response {
		if v.Domain != domain {
			continue
		}
		if len(v.Services) == 0 {
			return nil, nil
		}
		for k, v := range v.Services {
			v.Call = k
		}
		return slices.Collect(maps.Values(v.Services)), nil
	}

	return nil, schema.ErrNotFound.Withf("domain not found: %q", domain)
}

// Call a service for an entity. The serviceData map is sent as the JSON request
// body and typically includes "entity_id" plus any service-specific fields.
// Returns a list of states that changed while the service was being executed.
func (c *Client) Call(ctx context.Context, domain, service string, serviceData map[string]any) ([]*haschema.State, error) {
	if domain == "" {
		return nil, schema.ErrBadParameter.With("domain is required")
	}
	if service == "" {
		return nil, schema.ErrBadParameter.With("service is required")
	}

	if serviceData == nil {
		serviceData = map[string]any{}
	}
	payload, err := client.NewJSONRequest(serviceData)
	if err != nil {
		return nil, err
	}

	var response []*haschema.State
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("services", domain, service)); err != nil {
		return nil, err
	}

	return response, nil
}

// CallWithResponse calls a service and returns both changed states and service
// response data. Use this for services that support returning response data
// (e.g. weather.get_forecasts).
func (c *Client) CallWithResponse(ctx context.Context, domain, service string, serviceData map[string]any) (*CallResponse, error) {
	if domain == "" {
		return nil, schema.ErrBadParameter.With("domain is required")
	}
	if service == "" {
		return nil, schema.ErrBadParameter.With("service is required")
	}

	if serviceData == nil {
		serviceData = map[string]any{}
	}
	payload, err := client.NewJSONRequest(serviceData)
	if err != nil {
		return nil, err
	}

	var response CallResponse
	if err := c.DoWithContext(ctx, payload, &response,
		client.OptPath("services", domain, service),
		client.OptQuery(url.Values{"return_response": []string{""}}),
	); err != nil {
		return nil, err
	}

	return &response, nil
}
