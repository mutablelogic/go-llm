//go:build !client

package main

import (
	"os"
	"path/filepath"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	memory "github.com/mutablelogic/go-llm/memory"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_MemoryStore(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), memoryFile)
	file, err := newJSONFile(path)
	require.NoError(t, err)
	store := newMemoryStore(file)

	// With no file, nothing is known
	details, err := store.Details(ctx)
	require.NoError(t, err)
	assert.Equal(t, schema.Details{}, details)

	// The details are kept in the file, beside what else is in it, and read
	// by a memory, as it starts
	require.NoError(t, file.Set("other", "value"))
	m, err := memory.New(ctx, store)
	require.NoError(t, err)
	_, err = m.SetDetail(ctx, schema.FieldName, &schema.Detail{Value: "David"})
	require.NoError(t, err)
	_, err = m.SetDetail(ctx, schema.FieldLocation, &schema.Detail{Declined: true})
	require.NoError(t, err)

	file, err = newJSONFile(path)
	require.NoError(t, err)
	m, err = memory.New(ctx, newMemoryStore(file))
	require.NoError(t, err)
	assert.Equal(t, "David", m.Details().Get(schema.FieldName).Value)
	assert.True(t, m.Details().Get(schema.FieldLocation).Declined)
	assert.Equal(t, "value", file.Get("other"))

	// Details which aren't valid aren't read, or written
	require.NoError(t, os.WriteFile(path, []byte(`{"details":{"timezone":{"value":"Nowhere"}}}`), 0o600))
	_, err = store.Details(ctx)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	var details2 schema.Details
	details2.Timezone = &schema.Detail{Value: "Nowhere"}
	assert.ErrorIs(t, store.PutDetails(ctx, details2), llm.ErrBadParameter)
}
