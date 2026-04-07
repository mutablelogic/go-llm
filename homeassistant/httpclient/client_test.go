package homeassistant_test

import (
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	assert "github.com/stretchr/testify/assert"
)

func Test_client_001(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)
	t.Log(client)
}

///////////////////////////////////////////////////////////////////////////////
// ENVIRONMENT

func GetApiKey(t *testing.T) string {
	key := os.Getenv("HA_TOKEN")
	if key == "" {
		t.Skip("HA_TOKEN not set")
		t.SkipNow()
	}
	return key
}

func GetEndPoint(t *testing.T) string {
	key := os.Getenv("HA_ENDPOINT")
	if key == "" {
		t.Skip("HA_ENDPOINT not set")
		t.SkipNow()
	}
	return key
}
