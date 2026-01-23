package newsapi_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	assert "github.com/stretchr/testify/assert"
)

func Test_articles_001(t *testing.T) {
	assert := assert.New(t)

	articles, err := client.Headlines(context.Background(), newsapi.OptQuery("google"))
	assert.NoError(err)
	assert.NotNil(articles)

	body, _ := json.MarshalIndent(articles, "", "  ")
	t.Log(string(body))
}

func Test_articles_002(t *testing.T) {
	assert := assert.New(t)

	articles, err := client.Articles(context.Background(), newsapi.OptQuery("google"), newsapi.OptLimit(1))
	assert.NoError(err)
	assert.NotNil(articles)

	body, _ := json.MarshalIndent(articles, "", "  ")
	t.Log(string(body))
}
