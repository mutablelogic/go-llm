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

func Test_sources_001(t *testing.T) {
	assert := assert.New(t)
	client, err := newsapi.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	sources, err := client.Sources(newsapi.OptLanguage("en"))
	assert.NoError(err)
	assert.NotNil(sources)

	body, err := json.MarshalIndent(sources, "", "  ")
	t.Log(string(body))
}
