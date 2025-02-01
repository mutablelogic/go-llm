package newsapi

import (
	"context"
	"fmt"
	"slices"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type headlines struct {
	*Client `json:"-"`
	// CountryCode string `json:"country_code,omitempty" help:"The two-letter countrycode to return headlines for. Leave empty for worldwide headlines."`
}

type search struct {
	*Client `json:"-"`
	Query   string `json:"query" help:"A phrase used to search for news headlines." required:"true"`
}

type category struct {
	*Client  `json:"-"`
	Category string `json:"category" enum:"business, entertainment, health, science, sports, technology" help:"business, entertainment, health, science, sports, technology" required:"true"`
}

var _ llm.Tool = (*headlines)(nil)

var (
	categories = []string{"business", "entertainment", "health", "science", "sports", "technology"}
)

///////////////////////////////////////////////////////////////////////////////
// HEADLINES

func (headlines) Name() string {
	return "news_headlines"
}

func (headlines) Description() string {
	return "Return the current global news headlines"
}

func (headlines *headlines) Run(ctx context.Context) (any, error) {
	return headlines.Headlines(OptCategory("general"), OptLimit(10))
}

///////////////////////////////////////////////////////////////////////////////
// SEARCH

func (search) Name() string {
	return "news_search"
}

func (search) Description() string {
	return "Search the news archive with a search query"
}

func (search *search) Run(ctx context.Context) (any, error) {
	if search.Query == "" {
		return nil, nil
	}
	fmt.Printf("search for %q\n", search.Query)
	return search.Articles(OptQuery(search.Query), OptLimit(10))
}

///////////////////////////////////////////////////////////////////////////////
// CATEGORY

func (category) Name() string {
	return "news_headlines_category"
}

func (category) Description() string {
	return "Return the news headlines for a specific category"
}

func (category *category) Run(ctx context.Context) (any, error) {
	if !slices.Contains(categories, category.Category) {
		fmt.Printf("search for %q\n", category.Category)
		return category.Articles(OptQuery(category.Category), OptLimit(10))
	}
	fmt.Printf("category for %q\n", category.Category)
	return category.Headlines(OptCategory(category.Category), OptLimit(10))
}

/*
// Return all the agent tools for the weatherapi
func (c *Client) Tools() []agent.Tool {
	return []agent.Tool{
		&tool{
			name:        "current_headlines",
			description: "Return the current news headlines",
			run:         c.agentCurrentHeadlines,
		}, &tool{
			name:        "current_headlines_country",
			description: "Return the current news headlines for a country",
			run:         c.agentCountryHeadlines,
			params: []agent.ToolParameter{
				{
					Name:        "countrycode",
					Description: "The two-letter country code to return headlines for",
					Required:    true,
				},
			},
		}, &tool{
			name:        "current_headlines_category",
			description: "Return the current news headlines for a business, entertainment, health, science, sports or technology",
			run:         c.agentCategoryHeadlines,
			params: []agent.ToolParameter{
				{
					Name:        "category",
					Description: "business, entertainment, health, science, sports, technology",
					Required:    true,
				},
			},
		}, &tool{
			name:        "search_news",
			description: "Return the news headlines with a search query",
			run:         c.agentSearchNews,
			params: []agent.ToolParameter{
				{
					Name:        "query",
					Description: "A phrase used to search for news headlines",
					Required:    true,
				},
			},
		},
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - TOOL

func (*tool) Provider() string {
	return "newsapi"
}

func (t *tool) Name() string {
	return t.name
}

func (t *tool) Description() string {
	return t.description
}

func (t *tool) Params() []agent.ToolParameter {
	return t.params
}

func (t *tool) Run(ctx context.Context, call *agent.ToolCall) (*agent.ToolResult, error) {
	return t.run(ctx, call)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - TOOL

// Return the current general headlines
func (c *Client) agentCurrentHeadlines(_ context.Context, call *agent.ToolCall) (*agent.ToolResult, error) {
	response, err := c.Headlines(OptCategory("general"), OptLimit(5))
	if err != nil {
		return nil, err
	}
	return &agent.ToolResult{
		Id: call.Id,
		Result: map[string]any{
			"type":      "text",
			"headlines": response,
		},
	}, nil
}

// Return the headlines for a specific country
func (c *Client) agentCountryHeadlines(_ context.Context, call *agent.ToolCall) (*agent.ToolResult, error) {
	country, err := call.String("countrycode")
	if err != nil {
		return nil, err
	}
	country = strings.ToLower(country)
	response, err := c.Headlines(OptCountry(country), OptLimit(5))
	if err != nil {
		return nil, err
	}
	return &agent.ToolResult{
		Id: call.Id,
		Result: map[string]any{
			"type":      "text",
			"country":   country,
			"headlines": response,
		},
	}, nil
}

// Return the headlines for a specific category
func (c *Client) agentCategoryHeadlines(_ context.Context, call *agent.ToolCall) (*agent.ToolResult, error) {
	category, err := call.String("category")
	if err != nil {
		return nil, err
	}
	category = strings.ToLower(category)
	response, err := c.Headlines(OptCategory(category), OptLimit(5))
	if err != nil {
		return nil, err
	}
	return &agent.ToolResult{
		Id: call.Id,
		Result: map[string]any{
			"type":      "text",
			"category":  category,
			"headlines": response,
		},
	}, nil
}

// Return the headlines for a specific query
func (c *Client) agentSearchNews(_ context.Context, call *agent.ToolCall) (*agent.ToolResult, error) {
	query, err := call.String("query")
	if err != nil {
		return nil, err
	}
	response, err := c.Articles(OptQuery(query), OptLimit(5))
	if err != nil {
		return nil, err
	}
	return &agent.ToolResult{
		Id: call.Id,
		Result: map[string]any{
			"type":      "text",
			"query":     query,
			"headlines": response,
		},
	}, nil
}
*/
