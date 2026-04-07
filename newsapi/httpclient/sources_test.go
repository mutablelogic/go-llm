package newsapi_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	"github.com/stretchr/testify/assert"
)

func Test_sources_001(t *testing.T) {
	assert := assert.New(t)

	sources, err := client.Sources(context.Background(), &newsapi.SourcesRequest{
		Language: "en",
	})
	assert.NoError(err)
	assert.NotNil(sources)

	body, _ := json.MarshalIndent(sources, "", "  ")
	t.Log(string(body))
}
