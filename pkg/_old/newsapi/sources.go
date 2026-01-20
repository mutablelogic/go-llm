package newsapi

import (
	// Packages
	"github.com/mutablelogic/go-client"

	// Namespace imports
	. "github.com/djthorpe/go-errors"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Source struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
	Category    string `json:"category,omitempty"`
	Language    string `json:"language,omitempty"`
	Country     string `json:"country,omitempty"`
}

type respSources struct {
	Status  string   `json:"status"`
	Code    string   `json:"code,omitempty"`
	Message string   `json:"message,omitempty"`
	Sources []Source `json:"sources"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Sources returns all the models. The options which can be passed are:
//
//	  OptCategory: The category you would like to get sources for. Possible
//				  options are business, entertainment, general, health, science, sports,
//				  technology.
//
//	  OptLanguage: The language you would like to get sources for
//
//	  OptCountry: The country you would like to get sources for
func (c *Client) Sources(opt ...Opt) ([]Source, error) {
	var response respSources
	var query opts

	// Add options
	for _, opt := range opt {
		if err := opt(&query); err != nil {
			return nil, err
		}
	}

	// Request -> Response
	if err := c.Do(nil, &response, client.OptPath("top-headlines/sources"), client.OptQuery(query.Values())); err != nil {
		return nil, err
	} else if response.Status != "ok" {
		return nil, ErrBadParameter.Withf("%s: %s", response.Code, response.Message)
	}

	// Return success
	return response.Sources, nil
}
