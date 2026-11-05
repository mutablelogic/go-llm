package weatherapi

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/weatherapi/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type currentWeather struct {
	toolkit.Base
	client *httpclient.Client
}

type forecastWeather struct {
	toolkit.Base
	client *httpclient.Client
}

type alertsWeather struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*currentWeather)(nil)
var _ llm.Tool = (*forecastWeather)(nil)
var _ llm.Tool = (*alertsWeather)(nil)

////////////////////////////////////////////////////////////////////////////////
// CURRENT WEATHER

func (*currentWeather) Name() string {
	return "current"
}

func (*currentWeather) Description() string {
	return "Get current weather conditions for a location including temperature, wind, humidity, and precipitation."
}

// InputSchema returns the JSON schema for the tool input
func (*currentWeather) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.CurrentWeatherRequest]()
}

// Run the tool with the input
func (c *currentWeather) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.CurrentWeatherRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	// Validate required fields
	if req.Query == "" {
		return nil, llm.ErrBadParameter.With("query is required")
	}

	toolkit.Logf(ctx, "Getting the current weather for %s", req.Query)
	return jsonResult(c.client.Current(ctx, &req))
}

////////////////////////////////////////////////////////////////////////////////
// FORECAST WEATHER

func (*forecastWeather) Name() string {
	return "forecast"
}

func (*forecastWeather) Description() string {
	return "Get weather forecast for up to 14 days including daily and hourly forecasts, alerts, and air quality data."
}

// InputSchema returns the JSON schema for the tool input
func (*forecastWeather) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.ForecastWeatherRequest]()
}

// Run the tool with the input
func (f *forecastWeather) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.ForecastWeatherRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	// Validate required fields
	if req.Query == "" {
		return nil, llm.ErrBadParameter.With("query is required")
	}
	if req.Days < 1 || req.Days > 14 {
		return nil, llm.ErrBadParameter.With("days must be between 1 and 14")
	}

	if req.Date != "" {
		toolkit.Logf(ctx, "Getting the weather forecast for %s on %s", req.Query, req.Date)
	} else if req.Days == 1 {
		toolkit.Logf(ctx, "Getting today's weather forecast for %s", req.Query)
	} else {
		toolkit.Logf(ctx, "Getting the %d-day weather forecast for %s", req.Days, req.Query)
	}
	return jsonResult(f.client.Forecast(ctx, &req))
}

////////////////////////////////////////////////////////////////////////////////
// ALERTS WEATHER

func (*alertsWeather) Name() string {
	return "alerts"
}

func (*alertsWeather) Description() string {
	return "Get weather alerts and warnings for a location issued by government agencies."
}

// InputSchema returns the JSON schema for the tool input
func (*alertsWeather) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[httpclient.AlertsWeatherRequest]()
}

// Run the tool with the input
func (a *alertsWeather) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req httpclient.AlertsWeatherRequest
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}

	// Validate required fields
	if req.Query == "" {
		return nil, llm.ErrBadParameter.With("query is required")
	}

	toolkit.Logf(ctx, "Checking for weather alerts in %s", req.Query)
	return jsonResult(a.client.Alerts(ctx, &req))
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// unmarshal decodes the tool input, which may be empty, into v
func unmarshal(input json.RawMessage, v any) error {
	if len(input) == 0 {
		return nil
	}
	if err := json.Unmarshal(input, v); err != nil {
		return llm.ErrBadParameter.Withf("invalid input: %v", err)
	}
	return nil
}

// jsonResult returns the response from WeatherAPI as the tool's JSON result,
// or the error
func jsonResult(response any, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
