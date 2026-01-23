package newsapi

import (
	"fmt"
	"net/url"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ArticlesRequest struct {
	Query          string `json:"q,omitempty" jsonschema:"Search keywords or phrases"`
	SearchIn       string `json:"searchIn,omitempty" jsonschema:"The fields to restrict your q search to. Possible options: title, description, content (comma-separated)"`
	Sources        string `json:"sources,omitempty" jsonschema:"A comma-separated string of identifiers for the news sources or blogs you want headlines from (max 20)"`
	Domains        string `json:"domains,omitempty" jsonschema:"A comma-separated string of domains (eg bbc.co.uk, techcrunch.com) to restrict the search to"`
	ExcludeDomains string `json:"excludeDomains,omitempty" jsonschema:"A comma-separated string of domains to remove from the results"`
	From           string `json:"from,omitempty" jsonschema:"A date and optional time for the oldest article allowed. ISO 8601 format (e.g. 2026-01-23 or 2026-01-23T08:43:09)"`
	To             string `json:"to,omitempty" jsonschema:"A date and optional time for the newest article allowed. ISO 8601 format"`
	Language       string `json:"language,omitempty" jsonschema:"The 2-letter ISO-639-1 code of the language you want to get articles for"`
	SortBy         string `json:"sortBy,omitempty" jsonschema:"The order to sort the articles in. Possible options: relevancy, popularity, publishedAt"`
	PageSize       int    `json:"pageSize,omitempty" jsonschema:"The number of results to return per page (max 100)"`
	Page           int    `json:"page,omitempty" jsonschema:"Use this to page through the results"`
}

type HeadlinesRequest struct {
	Query    string `json:"q,omitempty" jsonschema:"Keywords or phrases to search for in the article title and body"`
	Category string `json:"category,omitempty" jsonschema:"The category you want to get headlines for. Possible options: business, entertainment, general, health, science, sports, technology"`
	Country  string `json:"country,omitempty" jsonschema:"The 2-letter ISO 3166-1 code of the country you want to get headlines for"`
	Sources  string `json:"sources,omitempty" jsonschema:"A comma-separated string of identifiers for the news sources or blogs you want headlines from. Cannot be mixed with country or category parameters"`
	Language string `json:"language,omitempty" jsonschema:"The 2-letter ISO-639-1 code of the language you want to get headlines for"`
	PageSize int    `json:"pageSize,omitempty" jsonschema:"The number of results to return per page (max 100)"`
	Page     int    `json:"page,omitempty" jsonschema:"Use this to page through the results"`
}

type SourcesRequest struct {
	Category string `json:"category,omitempty" jsonschema:"The category you want to get sources for. Possible options: business, entertainment, general, health, science, sports, technology"`
	Language string `json:"language,omitempty" jsonschema:"The 2-letter ISO-639-1 code of the language you want to get sources for"`
	Country  string `json:"country,omitempty" jsonschema:"The 2-letter ISO 3166-1 code of the country you want to get sources for"`
}

///////////////////////////////////////////////////////////////////////////////
// METHODS

func (r *ArticlesRequest) Values() url.Values {
	result := url.Values{}
	if r.Query != "" {
		result.Set("q", r.Query)
	}
	if r.SearchIn != "" {
		result.Set("searchIn", r.SearchIn)
	}
	if r.Sources != "" {
		result.Set("sources", r.Sources)
	}
	if r.Domains != "" {
		result.Set("domains", r.Domains)
	}
	if r.ExcludeDomains != "" {
		result.Set("excludeDomains", r.ExcludeDomains)
	}
	if r.From != "" {
		result.Set("from", r.From)
	}
	if r.To != "" {
		result.Set("to", r.To)
	}
	if r.Language != "" {
		result.Set("language", r.Language)
	}
	if r.SortBy != "" {
		result.Set("sortBy", r.SortBy)
	}
	if r.PageSize > 0 {
		result.Set("pageSize", fmt.Sprint(r.PageSize))
	}
	if r.Page > 0 {
		result.Set("page", fmt.Sprint(r.Page))
	}
	return result
}

func (r *HeadlinesRequest) Values() url.Values {
	result := url.Values{}
	if r.Query != "" {
		result.Set("q", r.Query)
	}
	if r.Category != "" {
		result.Set("category", r.Category)
	}
	if r.Country != "" {
		result.Set("country", r.Country)
	}
	if r.Sources != "" {
		result.Set("sources", r.Sources)
	}
	if r.Language != "" {
		result.Set("language", r.Language)
	}
	if r.PageSize > 0 {
		result.Set("pageSize", fmt.Sprint(r.PageSize))
	}
	if r.Page > 0 {
		result.Set("page", fmt.Sprint(r.Page))
	}
	return result
}

func (r *SourcesRequest) Values() url.Values {
	result := url.Values{}
	if r.Category != "" {
		result.Set("category", r.Category)
	}
	if r.Language != "" {
		result.Set("language", r.Language)
	}
	if r.Country != "" {
		result.Set("country", r.Country)
	}
	return result
}
