package newsapi

import (
	"time"

	// Packages
	"github.com/mutablelogic/go-client"

	// Namespace imports
	. "github.com/djthorpe/go-errors"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Article struct {
	Source      Source    `json:"source"`
	Title       string    `json:"title"`
	Author      string    `json:"author,omitempty"`
	Description string    `json:"description,omitempty"`
	Url         string    `json:"url,omitempty"`
	ImageUrl    string    `json:"urlToImage,omitempty"`
	PublishedAt time.Time `json:"publishedAt,omitempty"`
	Content     string    `json:"content,omitempty"`
}

type respArticles struct {
	Status       string    `json:"status"`
	Code         string    `json:"code,omitempty"`
	Message      string    `json:"message,omitempty"`
	TotalResults int       `json:"totalResults"`
	Articles     []Article `json:"articles"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Returns headlines
func (c *Client) Headlines(opt ...Opt) ([]Article, error) {
	var response respArticles
	var query opts

	// Add options
	for _, opt := range opt {
		if err := opt(&query); err != nil {
			return nil, err
		}
	}

	// Request -> Response
	if err := c.Do(nil, &response, client.OptPath("top-headlines"), client.OptQuery(query.Values())); err != nil {
		return nil, err
	} else if response.Status != "ok" {
		return nil, ErrBadParameter.Withf("%s: %s", response.Code, response.Message)
	}

	// Return success
	return response.Articles, nil
}

// Returns articles
func (c *Client) Articles(opt ...Opt) ([]Article, error) {
	var response respArticles
	var query opts

	// Add options
	for _, opt := range opt {
		if err := opt(&query); err != nil {
			return nil, err
		}
	}

	// Request -> Response
	if err := c.Do(nil, &response, client.OptPath("everything"), client.OptQuery(query.Values())); err != nil {
		return nil, err
	} else if response.Status != "ok" {
		return nil, ErrBadParameter.Withf("%s: %s", response.Code, response.Message)
	}

	// Return success
	return response.Articles, nil
}
