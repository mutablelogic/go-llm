package newsapi

import (
	"context"
	"fmt"
	"slices"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// HEADLINES

type headlines struct {
	*Client `json:"-"`
}

var _ llm.Tool = (*headlines)(nil)

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

type search struct {
	*Client `json:"-"`
	Query   string `json:"query" help:"A phrase used to search for news headlines." required:"true"`
}

var _ llm.Tool = (*search)(nil)

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
	fmt.Printf("  => Search for %q\n", search.Query)
	return search.Articles(OptQuery(search.Query), OptLimit(10))
}

///////////////////////////////////////////////////////////////////////////////
// CATEGORY

type category struct {
	*Client  `json:"-"`
	Category string `json:"category" enum:"business, entertainment, health, science, sports, technology" help:"business, entertainment, health, science, sports, technology" required:"true"`
}

var _ llm.Tool = (*category)(nil)

var (
	categories = []string{"business", "entertainment", "health", "science", "sports", "technology"}
)

func (category) Name() string {
	return "news_headlines_category"
}

func (category) Description() string {
	return "Return the news headlines for a specific category"
}

func (category *category) Run(ctx context.Context) (any, error) {
	if !slices.Contains(categories, category.Category) {
		fmt.Printf("  => Search for %q\n", category.Category)
		return category.Articles(OptQuery(category.Category), OptLimit(10))
	}
	fmt.Printf("  => Headlines for %q\n", category.Category)
	return category.Headlines(OptCategory(category.Category), OptLimit(10))
}
