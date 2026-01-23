package newsapi_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	assert "github.com/stretchr/testify/assert"
)

func Test_sources_001(t *testing.T) {
	assert := assert.New(t)

	sources, err := client.Sources(context.Background(), newsapi.OptLanguage("en"))
	assert.NoError(err)
	assert.NotNil(sources)

	body, err := json.MarshalIndent(sources, "", "  ")
	t.Log(string(body))
}
