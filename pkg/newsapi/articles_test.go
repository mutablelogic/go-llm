package newsapi_test

import (
	"encoding/json"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	assert "github.com/stretchr/testify/assert"
)

func Test_articles_001(t *testing.T) {
	assert := assert.New(t)
	client, err := newsapi.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	articles, err := client.Headlines(newsapi.OptQuery("google"))
	assert.NoError(err)
	assert.NotNil(articles)

	body, _ := json.MarshalIndent(articles, "", "  ")
	t.Log(string(body))
}

func Test_articles_002(t *testing.T) {
	assert := assert.New(t)
	client, err := newsapi.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	articles, err := client.Articles(newsapi.OptQuery("google"), newsapi.OptLimit(1))
	assert.NoError(err)
	assert.NotNil(articles)

	body, _ := json.MarshalIndent(articles, "", "  ")
	t.Log(string(body))
}
