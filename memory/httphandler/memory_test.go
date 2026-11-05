package httphandler_test

import (
	"net/http"
	"testing"

	// Packages
	memory "github.com/mutablelogic/go-llm/memory"
	httphandler "github.com/mutablelogic/go-llm/memory/httphandler"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Memories(t *testing.T) {
	m, err := memory.New(t.Context(), memory.NewStore())
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(m, router)
	})

	// Remembered, which is created, and then replaced, at its version
	var got schema.Memory
	assert.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPut, "memory/memory/family", map[string]any{"description": "The person's family", "body": "- Sister Cathy\n- Brother Tom"}, &got))
	assert.Equal(t, uint64(1), got.Version)
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodPut, "memory/memory/family", map[string]any{"description": "The person's family", "body": "- Sister Cathy, in Tokyo\n- Brother Tom", "version": 1}, &got))
	assert.Equal(t, uint64(2), got.Version)
	assert.Equal(t, http.StatusConflict, server.JSON(t, http.MethodPut, "memory/memory/family", map[string]any{"description": "x", "body": "x", "version": 1}, nil))
	assert.Equal(t, http.StatusConflict, server.JSON(t, http.MethodPut, "memory/memory/family", map[string]any{"description": "x", "body": "x"}, nil))
	assert.Equal(t, http.StatusCreated, server.JSON(t, http.MethodPut, "memory/memory/units", map[string]any{"description": "Units", "body": "Metric"}, nil))

	// Listed, and searched, and got
	var list schema.MemoryList
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "memory/memory", nil, &list))
	assert.Equal(t, uint(2), list.Count)
	list = schema.MemoryList{}
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "memory/memory?q=tokyo&limit=5", nil, &list))
	require.Len(t, list.Body, 1)
	assert.Equal(t, "family", list.Body[0].Key)
	got = schema.Memory{}
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "memory/memory/family", nil, &got))
	assert.Equal(t, "- Sister Cathy, in Tokyo\n- Brother Tom", got.Body)

	// A line forgotten, and then the memory
	got = schema.Memory{}
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodDelete, "memory/memory/family?line=Brother%20Tom", nil, &got))
	assert.Equal(t, "- Sister Cathy, in Tokyo", got.Body)
	assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodDelete, "memory/memory/family?line=Brother%20Tom", nil, nil))
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodDelete, "memory/memory/family", nil, nil))
	assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodGet, "memory/memory/family", nil, nil))
	assert.Equal(t, http.StatusNotFound, server.JSON(t, http.MethodDelete, "memory/memory/family", nil, nil))

	// Invalid memories
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPut, "memory/memory/units2", map[string]any{"description": "Units"}, nil))
	assert.Equal(t, http.StatusBadRequest, server.JSON(t, http.MethodPut, "memory/memory/units2", map[string]any{"description": "two\nlines", "body": "x"}, nil))
}
