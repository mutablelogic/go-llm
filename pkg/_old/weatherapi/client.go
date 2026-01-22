/*
weatherapi implements an API client for WeatherAPI
https://www.weatherapi.com/docs/
*/
package weatherapi

import (
	"net/url"

	// Packages
	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/tool"
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
func (c *Client) Current(q string) (Weather, error) {
	var response Weather

	// Set defaults
	response.Query = q

	// Set query parameters
	query := url.Values{}
	query.Set("key", c.key)
	query.Set("q", q)

	// Request -> Response
	if err := c.Do(nil, &response, client.OptPath("current.json"), client.OptQuery(query)); err != nil {
		return Weather{}, err
	} else {
		return response, nil
	}
}

// Forecast weather
func (c *Client) Forecast(q string, opts ...Opt) (Forecast, error) {
	var request options
	var response Forecast

	// Set defaults
	request.Values = url.Values{}
	response.Query = q

	// Set options
	for _, opt := range opts {
		if err := opt(&request); err != nil {
			return Forecast{}, err
		}
	}

	// Set query parameters
	request.Set("key", c.key)
	request.Set("q", q)

	// Request -> Response
	if err := c.Do(nil, &response, client.OptPath("forecast.json"), client.OptQuery(request.Values)); err != nil {
		return Forecast{}, err
	} else {
		return response, nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (client *Client) RegisterWithToolKit(toolkit *tool.ToolKit) error {
	// Register tools
	if err := toolkit.Register(&current_weather{client, ""}); err != nil {
		return err
	}
	if err := toolkit.Register(&forecast_weather{client, "", 0}); err != nil {
		return err
	}

	// Return success
	return nil
}
