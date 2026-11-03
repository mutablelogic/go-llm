/*
httpclient implements an API client for the Home Assistant REST API
https://developers.home-assistant.io/docs/api/rest/
*/
package httpclient

import (
	"bytes"
	"context"
	"maps"
	"net/url"
	"slices"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client for the Home Assistant server at the endpoint, with a
// long-lived access token. The endpoint is the server's URL, such as
// "http://homeassistant.local:8123", or its API URL, which ends in "/api".
func New(endpoint, apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Check for missing API key
	if apiKey == "" {
		return nil, llm.ErrBadParameter.With("missing API key")
	}

	// The endpoint is the API URL, with a final slash
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, llm.ErrBadParameter.Withf("invalid endpoint %q", endpoint)
	}
	if path := strings.TrimSuffix(u.Path, "/"); !strings.HasSuffix(path, "/api") {
		u.Path = path + "/api"
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}

	// Create client, with the access token as a bearer token
	client, err := client.New(append(opts, client.OptEndpoint(u.String()), client.OptReqToken(client.Token{
		Scheme: client.Bearer,
		Value:  apiKey,
	}))...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - HEALTH AND CONFIG

// Health returns "API running." if the Home Assistant API is reachable
func (c *Client) Health(ctx context.Context) (string, error) {
	var response struct {
		Message string `json:"message"`
	}
	if err := c.DoWithContext(ctx, nil, &response); err != nil {
		return "", err
	}
	return response.Message, nil
}

// Config returns the server's configuration
func (c *Client) Config(ctx context.Context) (*Config, error) {
	var response Config
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("config")); err != nil {
		return nil, err
	}
	return &response, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - STATES

// States returns all the entities and their state
func (c *Client) States(ctx context.Context) ([]*State, error) {
	var response []*State
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("states")); err != nil {
		return nil, err
	}
	return response, nil
}

// State returns the state of an entity
func (c *Client) State(ctx context.Context, entityId string) (*State, error) {
	var response State
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("states", entityId)); err != nil {
		return nil, err
	}
	return &response, nil
}

// SetState creates or updates the state of an entity, with optional
// attributes, and returns the state. This sets the representation within
// Home Assistant, and doesn't communicate with the device; use Call to
// actuate a device.
func (c *Client) SetState(ctx context.Context, entityId, state string, attributes map[string]any) (*State, error) {
	req := map[string]any{
		"state": state,
	}
	if len(attributes) > 0 {
		req["attributes"] = attributes
	}
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response State
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("states", entityId)); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteState removes an entity
func (c *Client) DeleteState(ctx context.Context, entityId string) error {
	return c.DoWithContext(ctx, client.MethodDelete, nil, client.OptPath("states", entityId))
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SERVICES

// Domains returns all domains and their services
func (c *Client) Domains(ctx context.Context) ([]*Domain, error) {
	var response []*Domain
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("services")); err != nil {
		return nil, err
	}
	return response, nil
}

// Services returns the services which can be called for a domain, in order of
// name. Returns ErrNotFound if there is no domain with the name.
func (c *Client) Services(ctx context.Context, domain string) ([]*Service, error) {
	domains, err := c.Domains(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		if d.Domain != domain {
			continue
		}
		result := make([]*Service, 0, len(d.Services))
		for _, name := range slices.Sorted(maps.Keys(d.Services)) {
			service := d.Services[name]
			service.Call = name
			result = append(result, service)
		}
		return result, nil
	}
	return nil, llm.ErrNotFound.Withf("domain %q", domain)
}

// Call calls a service of a domain. The service data is sent as the request
// body, and typically includes "entity_id" and any service-specific fields.
// Returns the states which changed while the service was being executed.
func (c *Client) Call(ctx context.Context, domain, service string, serviceData map[string]any) ([]*State, error) {
	if domain == "" {
		return nil, llm.ErrBadParameter.With("domain is required")
	} else if service == "" {
		return nil, llm.ErrBadParameter.With("service is required")
	}
	if serviceData == nil {
		serviceData = map[string]any{}
	}
	payload, err := client.NewJSONRequest(serviceData)
	if err != nil {
		return nil, err
	}

	var response []*State
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("services", domain, service)); err != nil {
		return nil, err
	}
	return response, nil
}

// CallWithResponse calls a service of a domain, and returns the states which
// changed and the service's response data, for services which return data,
// such as weather.get_forecasts
func (c *Client) CallWithResponse(ctx context.Context, domain, service string, serviceData map[string]any) (*CallResponse, error) {
	if domain == "" {
		return nil, llm.ErrBadParameter.With("domain is required")
	} else if service == "" {
		return nil, llm.ErrBadParameter.With("service is required")
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

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - EVENTS

// Events returns the events, and the number of listeners for each
func (c *Client) Events(ctx context.Context) ([]Event, error) {
	var response []Event
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("events")); err != nil {
		return nil, err
	}
	return response, nil
}

// FireEvent fires an event of the type, with optional event data, and returns
// a confirmation message
func (c *Client) FireEvent(ctx context.Context, eventType string, eventData map[string]any) (string, error) {
	if eventData == nil {
		eventData = map[string]any{}
	}
	payload, err := client.NewJSONRequest(eventData)
	if err != nil {
		return "", err
	}

	var response struct {
		Message string `json:"message"`
	}
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("events", eventType)); err != nil {
		return "", err
	}
	return response.Message, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - TEMPLATES

// Template renders a Jinja2 template, and returns the result as text
// See https://www.home-assistant.io/docs/configuration/templating
func (c *Client) Template(ctx context.Context, template string) (string, error) {
	payload, err := client.NewJSONRequest(struct {
		Template string `json:"template"`
	}{Template: template})
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := c.DoWithContext(ctx, payload, &buf, client.OptPath("template")); err != nil {
		return "", err
	}
	return buf.String(), nil
}
