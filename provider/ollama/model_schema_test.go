package ollama

import (
	"testing"
	"time"

	schema "github.com/mutablelogic/go-llm/kernel/schema"
	assert "github.com/stretchr/testify/assert"
)

func TestModelToSchemaCopiesContextLength(t *testing.T) {
	assert := assert.New(t)
	c := &Client{}
	m := model{
		Name:       "llama3.2:latest",
		Model:      "llama3.2:latest",
		ModifiedAt: time.Unix(1710000000, 0),
		Details: ModelDetails{
			Family: "llama",
		},
		Info: ModelInfo{
			"llama.context_length": float64(8192),
		},
		Capabilities: []string{"completion", "vision", "tools"},
	}

	result := c.modelToSchema(m)

	if assert.NotNil(result.InputTokenLimit) {
		assert.EqualValues(8192, *result.InputTokenLimit)
	}
	assert.Equal(schema.ModelCapCompletion|schema.ModelCapVision|schema.ModelCapTools, result.Cap)
	assert.EqualValues(8192, result.Meta["llama.context_length"])
	assert.Equal(c.Name(), result.OwnedBy)
}

func TestModelToSchemaSupportsEmbeddingsCapability(t *testing.T) {
	assert := assert.New(t)
	c := &Client{}
	m := model{
		Name:         "all-minilm:latest",
		Capabilities: []string{"embeddings"},
	}

	result := c.modelToSchema(m)

	assert.Equal(schema.ModelCapEmbeddings, result.Cap)
	assert.Nil(result.InputTokenLimit)
	assert.Nil(result.OutputTokenLimit)
}

func TestContextLengthFromModelMatchesGenericSuffix(t *testing.T) {
	assert := assert.New(t)
	limit := contextLengthFromModel(model{
		Info: ModelInfo{
			"bert.context_length": 2048,
		},
	})

	if assert.NotNil(limit) {
		assert.EqualValues(2048, *limit)
	}
}

func TestModelToSchemaOmitsEmptyDetailMeta(t *testing.T) {
	assert := assert.New(t)
	c := &Client{}
	m := model{
		Name:       "x/flux2-klein:latest",
		Model:      "x/flux2-klein:latest",
		ModifiedAt: time.Unix(1712210675, 0),
		Details: ModelDetails{
			Format:            "safetensors",
			Family:            "",
			Families:          nil,
			ParameterSize:     "",
			QuantizationLevel: "",
		},
	}

	result := c.modelToSchema(m)

	if assert.NotNil(result.Meta) {
		assert.Equal("safetensors", result.Meta["format"])
		_, ok := result.Meta["family"]
		assert.False(ok)
		_, ok = result.Meta["families"]
		assert.False(ok)
		_, ok = result.Meta["parameter_size"]
		assert.False(ok)
		_, ok = result.Meta["quantization_level"]
		assert.False(ok)
	}
}
