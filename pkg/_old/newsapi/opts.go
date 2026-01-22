package newsapi

import (
	"fmt"
	"net/url"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type opts struct {
	Category string `json:"category,omitempty"`
	Language string `json:"language,omitempty"`
	Country  string `json:"country,omitempty"`
	Query    string `json:"q,omitempty"`
	Limit    int    `json:"pageSize,omitempty"`
	Sort     string `json:"sortBy,omitempty"`
}

// Opt is a function which can be used to set options on a request
type Opt func(*opts) error

///////////////////////////////////////////////////////////////////////////////
// METHODS

func (o *opts) Values() url.Values {
	result := url.Values{}
	if o.Category != "" {
		result.Set("category", o.Category)
	}
	if o.Language != "" {
		result.Set("language", o.Language)
	}
	if o.Country != "" {
		result.Set("country", o.Country)
	}
	if o.Query != "" {
		result.Set("q", o.Query)
	}
	if o.Limit > 0 {
		result.Set("pageSize", fmt.Sprint(o.Limit))
	}
	if o.Sort != "" {
		result.Set("sortBy", o.Sort)
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// Set the category
func OptCategory(v string) Opt {
	return func(o *opts) error {
		o.Category = v
		return nil
	}
}

// Set the language
func OptLanguage(v string) Opt {
	return func(o *opts) error {
		o.Language = v
		return nil
	}
}

// Set the country
func OptCountry(v string) Opt {
	return func(o *opts) error {
		o.Country = v
		return nil
	}
}

// Set the query
func OptQuery(v string) Opt {
	return func(o *opts) error {
		o.Query = v
		return nil
	}
}

// Set the number of results
func OptLimit(v int) Opt {
	return func(o *opts) error {
		o.Limit = v
		return nil
	}
}

// Sort for articles by relevancy
func OptSortByRelevancy() Opt {
	return func(o *opts) error {
		o.Sort = "relevancy"
		return nil
	}
}

// Sort for articles by popularity
func OptSortByPopularity() Opt {
	return func(o *opts) error {
		o.Sort = "popularity"
		return nil
	}
}

// Sort for articles by date
func OptSortByDate() Opt {
	return func(o *opts) error {
		o.Sort = "publishedAt"
		return nil
	}
}
