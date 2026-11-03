/*
httpclient implements an API client for WeatherAPI
https://www.weatherapi.com/docs/
*/
package httpclient

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	key string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint = "https://api.weatherapi.com/v1"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client for WeatherAPI with the API key. The endpoint can be
// replaced with client.OptEndpoint, for example for testing.
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Check for missing API key
	if apiKey == "" {
		return nil, llm.ErrBadParameter.With("missing API key")
	}

	// Create client, with the default endpoint first so it can be replaced
	client, err := client.New(append([]client.ClientOpt{client.OptEndpoint(endPoint)}, opts...)...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{
		Client: client,
		key:    apiKey,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Current returns the current weather for the location in the request
func (c *Client) Current(ctx context.Context, req *CurrentWeatherRequest) (Weather, error) {
	var response Weather

	// Set defaults
	response.Query = req.Query

	// Request -> Response
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("current.json"), client.OptQuery(req.Values(c.key))); err != nil {
		return Weather{}, err
	}

	// Set the times in the location's timezone
	response.localize()
	return response, nil
}

// Forecast returns the weather forecast for the location in the request
func (c *Client) Forecast(ctx context.Context, req *ForecastWeatherRequest) (Forecast, error) {
	var response Forecast

	// Set defaults
	response.Query = req.Query

	// Request -> Response
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("forecast.json"), client.OptQuery(req.Values(c.key))); err != nil {
		return Forecast{}, err
	}

	// Set the times in the location's timezone
	response.localize()
	return response, nil
}

// Alerts returns the weather alerts for the location in the request
func (c *Client) Alerts(ctx context.Context, req *AlertsWeatherRequest) (WeatherAlerts, error) {
	var response WeatherAlerts

	// Set defaults
	response.Query = req.Query

	// Request -> Response
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("alerts.json"), client.OptQuery(req.Values(c.key))); err != nil {
		return WeatherAlerts{}, err
	}

	// Set the times in the location's timezone
	response.localize()
	return response, nil
}
