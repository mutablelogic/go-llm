package newsapi_test

import (
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	newsapi "github.com/mutablelogic/go-llm/pkg/newsapi"
	assert "github.com/stretchr/testify/assert"
)

func Test_client_001(t *testing.T) {
	assert := assert.New(t)
	client, err := newsapi.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)
	t.Log(client)
}

///////////////////////////////////////////////////////////////////////////////
// ENVIRONMENT

func GetApiKey(t *testing.T) string {
	key := os.Getenv("NEWSAPI_KEY")
	if key == "" {
		t.Skip("NEWSAPI_KEY not set")
		t.SkipNow()
	}
	return key
}
