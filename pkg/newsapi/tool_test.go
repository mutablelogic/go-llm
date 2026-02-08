package newsapi_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

func Test_tool_001(t *testing.T) {
	assert := assert.New(t)
	assert.NotNil(tools)
	assert.Len(tools, 3)

	// Check tool names
	assert.Equal("newsapi_articles", tools[0].Name())
	assert.Equal("newsapi_headlines", tools[1].Name())
	assert.Equal("newsapi_sources", tools[2].Name())

	// Check that schemas are not nil
	for _, tool := range tools {
		schema, err := tool.Schema()
		assert.NoError(err)
		assert.NotNil(schema)
		t.Logf("%s: %s", tool.Name(), tool.Description())
	}
}

func Test_tool_002(t *testing.T) {
	assert := assert.New(t)

	// Create a toolkit
	toolkit, err := tool.NewToolkit(tools...)
	assert.NoError(err)
	assert.NotNil(toolkit)
}

func Test_tool_002a(t *testing.T) {
	assert := assert.New(t)

	// Test that enum values are present in schemas
	// Articles tool - check sortBy enum
	articlesSchema, err := tools[0].Schema()
	assert.NoError(err)
	assert.NotNil(articlesSchema)
	if sortBy, ok := articlesSchema.Properties["sortBy"]; ok && sortBy != nil {
		assert.NotNil(sortBy.Enum)
		assert.Contains(sortBy.Enum, "relevancy")
		assert.Contains(sortBy.Enum, "popularity")
		assert.Contains(sortBy.Enum, "publishedAt")
	}

	// Headlines tool - check category enum
	headlinesSchema, err := tools[1].Schema()
	assert.NoError(err)
	assert.NotNil(headlinesSchema)
	if category, ok := headlinesSchema.Properties["category"]; ok && category != nil {
		assert.NotNil(category.Enum)
		assert.Contains(category.Enum, "business")
		assert.Contains(category.Enum, "technology")
		assert.Len(category.Enum, 7)
	}

	// Sources tool - check category enum
	sourcesSchema, err := tools[2].Schema()
	assert.NoError(err)
	assert.NotNil(sourcesSchema)
	if category, ok := sourcesSchema.Properties["category"]; ok && category != nil {
		assert.NotNil(category.Enum)
		assert.Contains(category.Enum, "business")
		assert.Contains(category.Enum, "technology")
		assert.Len(category.Enum, 7)
	}
}

func Test_tool_003(t *testing.T) {
	assert := assert.New(t)

	// Test articles tool with valid input
	articlesTool := tools[0]
	reqJSON, _ := json.Marshal(&newsapi.ArticlesRequest{
		Query:    "artificial intelligence",
		Language: "en",
		PageSize: 5,
		SortBy:   "relevancy",
	})

	result, err := articlesTool.Run(context.Background(), json.RawMessage(reqJSON))
	assert.NoError(err)
	assert.NotNil(result)

	articles, ok := result.([]newsapi.Article)
	assert.True(ok)
	assert.NotEmpty(articles)

	body, _ := json.MarshalIndent(articles, "", "  ")
	t.Log(string(body))
}

func Test_tool_004(t *testing.T) {
	assert := assert.New(t)

	// Test headlines tool with valid input
	headlinesTool := tools[1]
	reqJSON, _ := json.Marshal(&newsapi.HeadlinesRequest{
		Query:    "technology",
		Country:  "us",
		Category: "technology",
		PageSize: 5,
	})

	result, err := headlinesTool.Run(context.Background(), json.RawMessage(reqJSON))
	assert.NoError(err)
	assert.NotNil(result)

	articles, ok := result.([]newsapi.Article)
	assert.True(ok)
	t.Logf("Found %d headlines", len(articles))

	body, _ := json.MarshalIndent(articles, "", "  ")
	t.Log(string(body))
}

func Test_tool_005(t *testing.T) {
	assert := assert.New(t)

	// Test sources tool with valid input
	sourcesTool := tools[2]
	reqJSON, _ := json.Marshal(&newsapi.SourcesRequest{
		Category: "technology",
		Language: "en",
		Country:  "us",
	})

	result, err := sourcesTool.Run(context.Background(), json.RawMessage(reqJSON))
	assert.NoError(err)
	assert.NotNil(result)

	sources, ok := result.([]newsapi.Source)
	assert.True(ok)
	assert.NotEmpty(sources)

	body, _ := json.MarshalIndent(sources, "", "  ")
	t.Log(string(body))
}

func Test_tool_006(t *testing.T) {
	assert := assert.New(t)

	// Test articles tool with invalid JSON input
	articlesTool := tools[0]
	result, err := articlesTool.Run(context.Background(), json.RawMessage(`invalid`))
	assert.Error(err)
	assert.Nil(result)
}

func Test_tool_007(t *testing.T) {
	assert := assert.New(t)

	// Test headlines tool with nil input
	headlinesTool := tools[1]
	result, err := headlinesTool.Run(context.Background(), nil)
	// Should either succeed with empty request or fail gracefully
	if err != nil {
		t.Logf("Expected error with nil input: %v", err)
	} else {
		assert.NotNil(result)
	}
}

func Test_tool_008(t *testing.T) {
	assert := assert.New(t)

	// Test sources tool with empty JSON object
	sourcesTool := tools[2]
	result, err := sourcesTool.Run(context.Background(), json.RawMessage(`{}`))
	assert.NoError(err)
	assert.NotNil(result)
}

func Test_tool_009(t *testing.T) {
	assert := assert.New(t)

	// Test articles tool with minimal input
	articlesTool := tools[0]
	reqJSON, _ := json.Marshal(&newsapi.ArticlesRequest{
		Query: "bitcoin",
	})

	result, err := articlesTool.Run(context.Background(), json.RawMessage(reqJSON))
	assert.NoError(err)
	assert.NotNil(result)

	articles, ok := result.([]newsapi.Article)
	assert.True(ok)
	t.Logf("Found %d articles", len(articles))
}

func Test_tool_010(t *testing.T) {
	assert := assert.New(t)

	// Test different sort options
	articlesTool := tools[0]

	sortOptions := []string{"relevancy", "popularity", "publishedAt"}
	for _, sortBy := range sortOptions {
		reqJSON, _ := json.Marshal(&newsapi.ArticlesRequest{
			Query:    "climate",
			SortBy:   sortBy,
			PageSize: 3,
		})

		result, err := articlesTool.Run(context.Background(), json.RawMessage(reqJSON))
		assert.NoError(err)
		assert.NotNil(result)

		articles, ok := result.([]newsapi.Article)
		assert.True(ok)
		t.Logf("Sort by %s: found %d articles", sortBy, len(articles))
	}
}
