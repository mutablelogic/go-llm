package weatherapi

import (
	"context"
	"encoding/json"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	"github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type currentWeather struct {
	client *Client
}

type forecastWeather struct {
	client *Client
}

type alertsWeather struct {
	client *Client
}

var _ tool.Tool = (*currentWeather)(nil)
var _ tool.Tool = (*forecastWeather)(nil)
var _ tool.Tool = (*alertsWeather)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewTools returns a slice of weather tools for use with LLM agents
func NewTools(apikey string, opts ...client.ClientOpt) ([]tool.Tool, error) {
	// Create a client
	client, err := New(apikey, opts...)
	if err != nil {
		return nil, err
	}

	return []tool.Tool{
		&currentWeather{client: client},
		&forecastWeather{client: client},
		&alertsWeather{client: client},
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// CURRENT WEATHER

func (*currentWeather) Name() string {
	return "weatherapi_current"
}

func (*currentWeather) Description() string {
	return "Get current weather conditions for a location including temperature, wind, humidity, and precipitation."
}

// Return the JSON schema for the tool input
func (*currentWeather) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[CurrentWeatherRequest](nil)
}

// Run the tool with the given input
func (c *currentWeather) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req CurrentWeatherRequest

	// Unmarshal JSON input if provided
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	// Validate required fields
	if req.Query == "" {
		return nil, llm.ErrBadParameter.With("query is required")
	}

	return c.client.Current(ctx, &req)
}

///////////////////////////////////////////////////////////////////////////////
// FORECAST WEATHER

func (*forecastWeather) Name() string {
	return "weatherapi_forecast"
}

func (*forecastWeather) Description() string {
	return "Get weather forecast for up to 14 days including daily and hourly forecasts, alerts, and air quality data."
}

// Return the JSON schema for the tool input
func (*forecastWeather) Schema() (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[ForecastWeatherRequest](nil)
	if err != nil {
		return nil, err
	}

	// Add validation constraints for days
	if daysField, ok := schema.Properties["days"]; ok && daysField != nil {
		min := float64(1)
		max := float64(14)
		daysField.Minimum = &min
		daysField.Maximum = &max
	}

	return schema, nil
}

// Run the tool with the given input
func (f *forecastWeather) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req ForecastWeatherRequest

	// Unmarshal JSON input if provided
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	// Validate required fields
	if req.Query == "" {
		return nil, llm.ErrBadParameter.With("query is required")
	}
	if req.Days < 1 || req.Days > 14 {
		return nil, llm.ErrBadParameter.With("days must be between 1 and 14")
	}

	return f.client.Forecast(ctx, &req)
}

///////////////////////////////////////////////////////////////////////////////
// ALERTS WEATHER

func (*alertsWeather) Name() string {
	return "weatherapi_alerts"
}

func (*alertsWeather) Description() string {
	return "Get weather alerts and warnings for a location issued by government agencies."
}

// Return the JSON schema for the tool input
func (*alertsWeather) Schema() (*jsonschema.Schema, error) {
	return jsonschema.For[AlertsWeatherRequest](nil)
}

// Run the tool with the given input
func (a *alertsWeather) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req AlertsWeatherRequest

	// Unmarshal JSON input if provided
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("failed to unmarshal input: %v", err)
		}
	}

	// Validate required fields
	if req.Query == "" {
		return nil, llm.ErrBadParameter.With("query is required")
	}

	return a.client.Alerts(ctx, &req)
}
