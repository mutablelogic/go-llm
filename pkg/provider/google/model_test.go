package google

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func Test_toSchema_001(t *testing.T) {
	// Test basic field mapping
	assert := assert.New(t)
	m := &geminiModel{
		Name:        "models/gemini-2.0-flash",
		DisplayName: "Gemini 2.0 Flash",
		Description: "Fast multimodal model",
		Version:     "2.0",
	}
	result := m.toSchema()

	assert.Equal("gemini-2.0-flash", result.Name)
	assert.Equal("Fast multimodal model", result.Description)
	assert.Equal("gemini", result.OwnedBy)
	assert.Equal("2.0", result.Meta["version"])
}

func Test_toSchema_002(t *testing.T) {
	// Test that DisplayName is used when Description is empty
	assert := assert.New(t)
	m := &geminiModel{
		Name:        "models/gemini-test",
		DisplayName: "Test Model Display",
	}
	result := m.toSchema()

	assert.Equal("Test Model Display", result.Description)
}

func Test_toSchema_003(t *testing.T) {
	// Test that models/ prefix is stripped from name
	assert := assert.New(t)
	m := &geminiModel{Name: "models/some-model"}
	result := m.toSchema()
	assert.Equal("some-model", result.Name)

	// Name without prefix should be left as-is
	m2 := &geminiModel{Name: "no-prefix-model"}
	result2 := m2.toSchema()
	assert.Equal("no-prefix-model", result2.Name)
}

func Test_toSchema_004(t *testing.T) {
	// Test that token limits and sampling parameters appear in meta
	assert := assert.New(t)
	m := &geminiModel{
		Name:             "models/test-model",
		InputTokenLimit:  1048576,
		OutputTokenLimit: 8192,
		Temperature:      1.0,
		MaxTemperature:   2.0,
		TopP:             0.95,
		TopK:             40,
	}
	result := m.toSchema()

	// JSON round-trip produces float64 values
	assert.EqualValues(1048576, result.Meta["inputTokenLimit"])
	assert.EqualValues(8192, result.Meta["outputTokenLimit"])
	assert.EqualValues(1.0, result.Meta["temperature"])
	assert.EqualValues(2.0, result.Meta["maxTemperature"])
	assert.EqualValues(0.95, result.Meta["topP"])
	assert.EqualValues(40, result.Meta["topK"])
}

func Test_toSchema_005(t *testing.T) {
	// Test that supported generation methods are preserved in meta
	assert := assert.New(t)
	m := &geminiModel{
		Name:                       "models/test-model",
		SupportedGenerationMethods: []string{"generateContent", "countTokens"},
	}
	result := m.toSchema()

	methods, ok := result.Meta["supportedGenerationMethods"].([]any)
	assert.True(ok)
	assert.Len(methods, 2)
	assert.Equal("generateContent", methods[0])
	assert.Equal("countTokens", methods[1])
}

func Test_toSchema_006(t *testing.T) {
	// Test that thinking flag appears in meta
	assert := assert.New(t)
	m := &geminiModel{
		Name:     "models/thinking-model",
		Thinking: true,
	}
	result := m.toSchema()
	assert.Equal(true, result.Meta["thinking"])
}

func Test_toSchema_007(t *testing.T) {
	// Test with minimal model (empty fields omitted by json omitempty)
	assert := assert.New(t)
	m := &geminiModel{
		Name: "models/minimal",
	}
	result := m.toSchema()

	assert.Equal("minimal", result.Name)
	assert.Equal("gemini", result.OwnedBy)
	assert.Empty(result.Description)
	// Zero-value fields should be omitted by json omitempty
	assert.Nil(result.Meta["inputTokenLimit"])
	assert.Nil(result.Meta["outputTokenLimit"])
	assert.Nil(result.Meta["temperature"])
	assert.Nil(result.Meta["thinking"])
}
