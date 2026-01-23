package schema_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnthropicMessage tests unmarshalling and remarshalling
// test data from the testdata/anthropic directory
func TestAnthropicMessage(t *testing.T) {
	testdataDir := filepath.Join("testdata", "anthropic")

	// Read all JSON files in the testdata directory
	entries, err := os.ReadDir(testdataDir)
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			require := require.New(t)

			// Read the test file
			path := filepath.Join(testdataDir, entry.Name())
			data, err := os.ReadFile(path)
			require.NoError(err)

			// Check if this is a meta file (containing examples array)
			var metaFile struct {
				Description string `json:"description"`
				Examples    []struct {
					Name    string          `json:"name"`
					Message json.RawMessage `json:"message"`
				} `json:"examples"`
			}

			if err := json.Unmarshal(data, &metaFile); err == nil && len(metaFile.Examples) > 0 {
				// This is a meta file with multiple examples
				for _, example := range metaFile.Examples {
					t.Run(example.Name, func(t *testing.T) {
						testRoundTrip(t, example.Message)
					})
				}
			} else {
				// This is a single message file
				testRoundTrip(t, data)
			}
		})
	}
}

// testRoundTrip tests that a message can be unmarshalled and remarshalled
func testRoundTrip(t *testing.T, jsonData []byte) {
	assert := assert.New(t)
	require := require.New(t)

	// Unmarshal the Anthropic format
	var am schema.AnthropicMessage
	err := json.Unmarshal(jsonData, &am)
	require.NoError(err, "failed to unmarshal: %s", string(jsonData))

	// Marshal back to JSON
	remarshalled, err := json.Marshal(am)
	require.NoError(err)

	// Unmarshal both for comparison
	var original, roundtrip map[string]interface{}
	err = json.Unmarshal(jsonData, &original)
	require.NoError(err)
	err = json.Unmarshal(remarshalled, &roundtrip)
	require.NoError(err)

	// Verify they're equivalent
	assert.Equal(original, roundtrip, "round-trip failed: original and remarshalled JSON differ")
}
