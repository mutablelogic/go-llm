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
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ClientMemories(t *testing.T) {
	ctx := t.Context()
	m, err := memory.New(ctx, memory.NewStore())
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(m, router)
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)

	got, err := c.RememberMemory(ctx, "cats", schema.MemoryRequest{Description: "Cathy and Petra's cats", Body: "- Otto\n- Karl"})
	require.NoError(t, err)
	assert.Len(t, got.Version, 12)
	_, err = c.RememberMemory(ctx, "cats", schema.MemoryRequest{Description: "Cats", Body: "- Otto"})
	assert.Error(t, err, "without its version")

	list, err := c.ListMemories(ctx, schema.MemoryListRequest{Q: "karl", Limit: types.Ptr(uint64(10))})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	got, err = c.GetMemory(ctx, "cats")
	require.NoError(t, err)
	assert.Equal(t, "- Otto\n- Karl", got.Body)

	got, err = c.ForgetMemory(ctx, "cats", "- Karl")
	require.NoError(t, err)
	assert.Equal(t, "- Otto", got.Body)
	_, err = c.ForgetMemory(ctx, "cats", "")
	require.NoError(t, err)
	_, err = c.GetMemory(ctx, "cats")
	assert.Error(t, err)
}
