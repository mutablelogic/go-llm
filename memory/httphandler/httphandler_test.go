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

func Test_Details(t *testing.T) {
	m, err := memory.New(t.Context(), memory.NewStore())
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(m, router)
	})

	// None are known
	var details schema.Details
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "memory/detail", nil, &details))
	assert.Equal(t, schema.Details{}, details)

	// Set a value, and decline one, which the memory has
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodPut, "memory/detail/location", map[string]any{"value": "Berlin"}, &details))
	assert.Equal(t, &schema.Detail{Value: "Berlin"}, details.Location)
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodPut, "memory/detail/name", map[string]any{"declined": true}, &details))
	assert.Equal(t, &schema.Detail{Declined: true}, details.Name)
	assert.Equal(t, details, m.Details())

	details = schema.Details{}
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodGet, "memory/detail", nil, &details))
	assert.Equal(t, m.Details(), details)

	// Forget one
	details = schema.Details{}
	assert.Equal(t, http.StatusOK, server.JSON(t, http.MethodDelete, "memory/detail/location", nil, &details))
	assert.Nil(t, details.Location)
	assert.Equal(t, &schema.Detail{Declined: true}, details.Name)

	// Invalid requests
	for _, req := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, "memory/detail/age", map[string]any{"value": "42"}},
		{http.MethodPut, "memory/detail/location", map[string]any{"value": "Berlin", "declined": true}},
		{http.MethodPut, "memory/detail/location", map[string]any{}},
		{http.MethodPut, "memory/detail/timezone", map[string]any{"value": "Nowhere"}},
		{http.MethodDelete, "memory/detail/age", nil},
	} {
		assert.Equal(t, http.StatusBadRequest, server.JSON(t, req.method, req.path, req.body, nil), req.path)
	}
}
