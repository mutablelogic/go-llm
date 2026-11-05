package httpclient_test

import (
	"testing"

	// Packages
	memory "github.com/mutablelogic/go-llm/memory"
	httpclient "github.com/mutablelogic/go-llm/memory/httpclient"
	httphandler "github.com/mutablelogic/go-llm/memory/httphandler"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Client(t *testing.T) {
	ctx := t.Context()
	m, err := memory.New(ctx, memory.NewStore())
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(m, router)
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)

	details, err := c.GetDetails(ctx)
	require.NoError(t, err)
	assert.Equal(t, schema.Details{}, *details)

	details, err = c.SetDetail(ctx, schema.FieldAssistant, schema.Detail{Value: "Nano"})
	require.NoError(t, err)
	assert.Equal(t, &schema.Detail{Value: "Nano"}, details.Assistant)
	details, err = c.SetDetail(ctx, schema.FieldLocation, schema.Detail{Declined: true})
	require.NoError(t, err)
	assert.Equal(t, &schema.Detail{Declined: true}, details.Location)

	details, err = c.ForgetDetail(ctx, schema.FieldAssistant)
	require.NoError(t, err)
	assert.Nil(t, details.Assistant)
	assert.Equal(t, m.Details(), *details)

	_, err = c.SetDetail(ctx, schema.FieldTimezone, schema.Detail{Value: "Nowhere"})
	assert.Error(t, err)
}
