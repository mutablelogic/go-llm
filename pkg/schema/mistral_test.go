package schema_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/stretchr/testify/require"
)

// TestMistralMessage tests unmarshalling and remarshalling
// test data from the testdata/mistral directory
func TestMistralMessage(t *testing.T) {
	testdataDir := filepath.Join("testdata", "mistral")

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
						testMistralRoundTrip(t, example.Message)
					})
				}
			} else {
				// This is a single message file
				testMistralRoundTrip(t, data)
			}
		})
	}
}

// testMistralRoundTrip tests that a message can be unmarshalled and remarshalled
func testMistralRoundTrip(t *testing.T, jsonData []byte) {
	require := require.New(t)

	// Unmarshal the Mistral format
	var mm schema.MistralMessage
	err := json.Unmarshal(jsonData, &mm)
	require.NoError(err, "failed to unmarshal: %s", string(jsonData))

	// Marshal back to JSON — should round-trip without error
	_, err = json.Marshal(mm)
	require.NoError(err, "failed to marshal after unmarshal")
}
