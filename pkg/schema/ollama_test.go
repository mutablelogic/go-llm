package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestOllamaMessage(t *testing.T) {
	testDataRoot := filepath.Join("testdata", "ollama")

	// Find all JSON test files
	files, err := filepath.Glob(filepath.Join(testDataRoot, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "No test files found in %s", testDataRoot)

	for _, file := range files {
		fileName := filepath.Base(file)
		t.Run(fileName, func(t *testing.T) {
			// Read test file
			data, err := os.ReadFile(file)
			require.NoError(t, err)

			// Parse test structure
			var testFile struct {
				Description string `json:"description"`
				Examples    []struct {
					Name    string          `json:"name"`
					Message json.RawMessage `json:"message"`
				} `json:"examples"`
			}
			require.NoError(t, json.Unmarshal(data, &testFile))

			// Test each example
			for _, example := range testFile.Examples {
				t.Run(example.Name, func(t *testing.T) {
					// Unmarshal the Ollama message
					var ollamaMsg OllamaMessage
					err := json.Unmarshal(example.Message, &ollamaMsg)
					require.NoError(t, err, "Failed to unmarshal Ollama message")

					// Marshal it back
					marshaled, err := json.Marshal(ollamaMsg)
					require.NoError(t, err, "Failed to marshal Ollama message")

					// Unmarshal again to verify round-trip
					var ollamaMsg2 OllamaMessage
					err = json.Unmarshal(marshaled, &ollamaMsg2)
					require.NoError(t, err, "Failed to unmarshal after round-trip")

					// Verify the messages are equivalent
					assert.Equal(t, ollamaMsg.Role, ollamaMsg2.Role, "Role mismatch after round-trip")
					assert.Equal(t, len(ollamaMsg.Content), len(ollamaMsg2.Content), "Content block count mismatch")
				})
			}
		})
	}
}
