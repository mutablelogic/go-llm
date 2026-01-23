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

// TestGeminiMessage tests unmarshalling and remarshalling
// test data from the testdata/gemini directory
func TestGeminiMessage(t *testing.T) {
	testdataDir := filepath.Join("testdata", "gemini")

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
						testGeminiRoundTrip(t, example.Message)
					})
				}
			} else {
				// This is a single message file
				testGeminiRoundTrip(t, data)
			}
		})
	}
}

// testGeminiRoundTrip tests that a message can be unmarshalled and remarshalled
func testGeminiRoundTrip(t *testing.T, jsonData []byte) {
	assert := assert.New(t)
	require := require.New(t)

	// Unmarshal the Gemini format
	var gm schema.GeminiMessage
	err := json.Unmarshal(jsonData, &gm)
	require.NoError(err, "failed to unmarshal: %s", string(jsonData))

	// Marshal back to JSON
	remarshalled, err := json.Marshal(gm)
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
