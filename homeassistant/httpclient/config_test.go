package homeassistant_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	assert "github.com/stretchr/testify/assert"
)

func Test_config_001(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)

	config, err := client.Config(context.Background())
	assert.NoError(err)
	assert.NotNil(config)

	t.Log(config)
	assert.NotEmpty(config.Version)
	assert.NotEmpty(config.LocationName)
}
