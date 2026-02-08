package anthropic

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
)

func Test_toSchema_001(t *testing.T) {
	// Test basic field mapping
	assert := assert.New(t)
	m := model{
		Id:          "claude-sonnet-4-20250514",
		DisplayName: "Claude Sonnet 4",
		Type:        "model",
		CreatedAt:   time.Date(2025, 5, 22, 0, 0, 0, 0, time.UTC),
	}
	result := m.toSchema()

	assert.Equal("claude-sonnet-4-20250514", result.Name)
	assert.Equal("Claude Sonnet 4", result.Description)
	assert.Equal("anthropic", result.OwnedBy)
	assert.Equal(time.Date(2025, 5, 22, 0, 0, 0, 0, time.UTC), result.Created)
	assert.Equal("sonnet", result.Meta["variant"])
	assert.Equal("4", result.Meta["version"])
	assert.Equal("20250514", result.Meta["date"])
}

func Test_toSchema_002(t *testing.T) {
	// Test that meta fields are omitted when parseModelId returns empty
	assert := assert.New(t)
	m := model{
		Id:          "some-unknown-model",
		DisplayName: "Unknown",
	}
	result := m.toSchema()

	assert.Equal("some-unknown-model", result.Name)
	assert.Equal("Unknown", result.Description)
	assert.Equal("anthropic", result.OwnedBy)
	assert.Empty(result.Meta)
}

func Test_toSchema_003(t *testing.T) {
	// Test old format with minor version (e.g. claude-3-5-haiku-20241022)
	assert := assert.New(t)
	m := model{
		Id:          "claude-3-5-haiku-20241022",
		DisplayName: "Claude Haiku 3.5",
	}
	result := m.toSchema()

	assert.Equal("haiku", result.Meta["variant"])
	assert.Equal("3.5", result.Meta["version"])
	assert.Equal("20241022", result.Meta["date"])
}

func Test_toSchema_004(t *testing.T) {
	// Test new format with minor version and no date (e.g. claude-opus-4-6)
	assert := assert.New(t)
	m := model{
		Id:          "claude-opus-4-6",
		DisplayName: "Claude Opus 4.6",
	}
	result := m.toSchema()

	assert.Equal("opus", result.Meta["variant"])
	assert.Equal("4.6", result.Meta["version"])
	// No date in this format
	_, hasDate := result.Meta["date"]
	assert.False(hasDate)
}

func Test_parseModelId_001(t *testing.T) {
	// Old format major only: claude-3-haiku-20240307
	variant, version, date := parseModelId("claude-3-haiku-20240307")
	assert.Equal(t, "haiku", variant)
	assert.Equal(t, "3", version)
	assert.Equal(t, "20240307", date)
}

func Test_parseModelId_002(t *testing.T) {
	// Old format with minor version: claude-3-5-haiku-20241022
	variant, version, date := parseModelId("claude-3-5-haiku-20241022")
	assert.Equal(t, "haiku", variant)
	assert.Equal(t, "3.5", version)
	assert.Equal(t, "20241022", date)
}

func Test_parseModelId_003(t *testing.T) {
	// New format major with date: claude-opus-4-20250514
	variant, version, date := parseModelId("claude-opus-4-20250514")
	assert.Equal(t, "opus", variant)
	assert.Equal(t, "4", version)
	assert.Equal(t, "20250514", date)
}

func Test_parseModelId_004(t *testing.T) {
	// New format minor with date: claude-opus-4-5-20251101
	variant, version, date := parseModelId("claude-opus-4-5-20251101")
	assert.Equal(t, "opus", variant)
	assert.Equal(t, "4.5", version)
	assert.Equal(t, "20251101", date)
}

func Test_parseModelId_005(t *testing.T) {
	// New format minor without date: claude-opus-4-6
	variant, version, date := parseModelId("claude-opus-4-6")
	assert.Equal(t, "opus", variant)
	assert.Equal(t, "4.6", version)
	assert.Equal(t, "", date)
}

func Test_parseModelId_006(t *testing.T) {
	// Unrecognised format returns empty strings
	variant, version, date := parseModelId("gpt-4o-mini")
	assert.Equal(t, "", variant)
	assert.Equal(t, "", version)
	assert.Equal(t, "", date)
}

func Test_parseModelId_007(t *testing.T) {
	// New format with different variants
	tests := []struct {
		id      string
		variant string
		version string
		date    string
	}{
		{"claude-sonnet-4-20250514", "sonnet", "4", "20250514"},
		{"claude-sonnet-4-5-20250929", "sonnet", "4.5", "20250929"},
		{"claude-haiku-4-5-20251001", "haiku", "4.5", "20251001"},
		{"claude-3-7-sonnet-20250219", "sonnet", "3.7", "20250219"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			variant, version, date := parseModelId(tt.id)
			assert.Equal(t, tt.variant, variant)
			assert.Equal(t, tt.version, version)
			assert.Equal(t, tt.date, date)
		})
	}
}
