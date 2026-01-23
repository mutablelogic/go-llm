/*
weatherapi implements an API client for WeatherAPI
https://www.weatherapi.com/docs/
*/
package weatherapi

import (
	"context"

	// Packages
	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
	key string
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endPoint = "https://api.weatherapi.com/v1"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client
func New(ApiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Check for missing API key
	if ApiKey == "" {
		return nil, llm.ErrBadParameter.With("missing API key")
	}
	// Create client
	opts = append(opts, client.OptEndpoint(endPoint))
	client, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{
		Client: client,
		key:    ApiKey,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Current weather
func (c *Client) Current(ctx context.Context, req *CurrentWeatherRequest) (Weather, error) {
	var response Weather

	// Set defaults
	response.Query = req.Query

	// Request -> Response
	if err := c.Do(nil, &response, client.OptPath("current.json"), client.OptQuery(req.Values(c.key))); err != nil {
		return Weather{}, err
	}

	return response, nil
}

// Forecast weather
func (c *Client) Forecast(ctx context.Context, req *ForecastWeatherRequest) (Forecast, error) {
	var response Forecast

	// Set defaults
	response.Query = req.Query

	// Request -> Response
	if err := c.Do(nil, &response, client.OptPath("forecast.json"), client.OptQuery(req.Values(c.key))); err != nil {
		return Forecast{}, err
	}

	return response, nil
}

// Alerts weather
func (c *Client) Alerts(ctx context.Context, req *AlertsWeatherRequest) (Forecast, error) {
	var response Forecast

	// Set defaults
	response.Query = req.Query

	// Request -> Response
	if err := c.Do(nil, &response, client.OptPath("forecast.json"), client.OptQuery(req.Values(c.key))); err != nil {
		return Forecast{}, err
	}

	return response, nil
}
