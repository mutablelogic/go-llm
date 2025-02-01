package mistral_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	mistral "github.com/mutablelogic/go-llm/pkg/mistral"
	assert "github.com/stretchr/testify/assert"
)

func Test_models_001(t *testing.T) {
	assert := assert.New(t)
	client, err := mistral.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)
	response, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(response)
	data, err := json.MarshalIndent(response, "", "  ")
	assert.NoError(err)
	t.Log(string(data))
}
