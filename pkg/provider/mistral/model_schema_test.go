package mistral

import (
	"testing"
	"time"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

func TestModelToSchemaCopiesTokenLimit(t *testing.T) {
	assert := assert.New(t)
	m := model{
		Id:               "mistral-small-latest",
		Description:      "Small chat model",
		Created:          1710000000,
		MaxContextLength: 32768,
	}

	result := m.toSchema()

	if assert.NotNil(result.InputTokenLimit) {
		assert.EqualValues(32768, *result.InputTokenLimit)
	}
	assert.Nil(result.OutputTokenLimit)
	assert.Equal(time.Unix(1710000000, 0), result.Created)
	assert.EqualValues(32768, result.Meta["max_context_length"])
}

func TestCapabilitiesToSchema(t *testing.T) {
	assert := assert.New(t)

	cap := capabilities{
		CompletionChat:     true,
		FunctionCalling:    true,
		Vision:             true,
		AudioTranscription: true,
	}

	assert.Equal(
		schema.ModelCapCompletion|schema.ModelCapTools|schema.ModelCapVision|schema.ModelCapTranscription,
		cap.toSchema(),
	)
}

func TestModelToSchemaLeavesEmptyLimitNil(t *testing.T) {
	assert := assert.New(t)
	m := model{Id: "mistral-embed"}

	result := m.toSchema()

	assert.Nil(result.InputTokenLimit)
	assert.Nil(result.OutputTokenLimit)
	assert.Equal(schema.ModelCapEmbeddings, result.Cap)
}

func TestModelToSchemaAddsEmbeddingsCapabilityFromName(t *testing.T) {
	assert := assert.New(t)
	m := model{
		Id: "codestral-embed-25-05",
		Capabilities: capabilities{
			CompletionChat: true,
		},
	}

	result := m.toSchema()

	assert.Equal(schema.ModelCapCompletion|schema.ModelCapEmbeddings, result.Cap)
}
