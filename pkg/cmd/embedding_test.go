package cmd

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestReadEmbeddingCSVUsesFirstColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.csv")
	require.NoError(t, os.WriteFile(path, []byte("alpha,1\nbeta,2,3\n\n"), 0o600))

	result, err := readEmbeddingCSV(path)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, []string{"alpha", "beta"}, result)
}

func TestWriteEmbeddingCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")
	err := writeEmbeddingCSV(path, []string{"alpha", "beta"}, [][]float64{{0.1, 0.2}, {0.3, 0.4}})
	if !assert.NoError(t, err) {
		return
	}

	file, err := os.Open(path)
	if !assert.NoError(t, err) {
		return
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, [][]string{{"alpha", "0.1", "0.2"}, {"beta", "0.3", "0.4"}}, rows)
}

func TestWriteEmbeddingCSVMismatchedRows(t *testing.T) {
	err := writeEmbeddingCSV(filepath.Join(t.TempDir(), "out.csv"), []string{"alpha"}, [][]float64{{0.1}, {0.2}})
	assert.Error(t, err)
}

func TestEmbeddingCommandInputFromArgs(t *testing.T) {
	input, err := (EmbeddingCommand{EmbeddingRequest: schema.EmbeddingRequest{Input: []string{"alpha", "beta"}}}).input()
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, []string{"alpha", "beta"}, input)
}

func TestEmbeddingCommandInputMutuallyExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.csv")
	require.NoError(t, os.WriteFile(path, []byte("alpha\n"), 0o600))

	_, err := (EmbeddingCommand{
		EmbeddingRequest: schema.EmbeddingRequest{Input: []string{"beta"}},
		CSV:              path,
	}).input()
	assert.Error(t, err)
}

func TestEmbeddingCommandOutputPath(t *testing.T) {
	assert.Equal(t, "", (EmbeddingCommand{}).outputPath())
	assert.Equal(t, "", (EmbeddingCommand{CSV: "/tmp/input.csv"}).outputPath())
	assert.Equal(t, "/tmp/out.csv", (EmbeddingCommand{Out: "/tmp/out.csv"}).outputPath())
}

func TestGetModelCommandDefaultKeys(t *testing.T) {
	modelKey, providerKey := (GetModelCommand{}).defaultKeys()
	assert.Equal(t, "model", modelKey)
	assert.Equal(t, "provider", providerKey)

	modelKey, providerKey = (GetModelCommand{Embedding: true}).defaultKeys()
	assert.Equal(t, "embedding_model", modelKey)
	assert.Equal(t, "embedding_provider", providerKey)
}
