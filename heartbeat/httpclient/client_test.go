package httpclient_test

import (
	"testing"
	"time"
	"uuid"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	httpclient "github.com/mutablelogic/go-llm/heartbeat/httpclient"
	httphandler "github.com/mutablelogic/go-llm/heartbeat/httphandler"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Client(t *testing.T) {
	ctx := t.Context()
	h, err := heartbeat.New(heartbeat.NewStore())
	require.NoError(t, err)
	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(h, router)
	})
	c, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)

	// Created, with a schedule, and a time
	session := uuid.New()
	daily, err := c.CreateHeartbeat(ctx, schema.HeartbeatMeta{Title: "Briefing", Prompt: "Brief me", Session: session, Schedule: "0 8 * * 1-5", Timezone: "Europe/Berlin"})
	require.NoError(t, err)
	require.NotNil(t, daily.Next)
	once, err := c.CreateHeartbeat(ctx, schema.HeartbeatMeta{Title: "Call", Prompt: "Remind them to call", Session: uuid.New(), At: types.Ptr(time.Now().Add(time.Hour))})
	require.NoError(t, err)
	_, err = c.CreateHeartbeat(ctx, schema.HeartbeatMeta{Title: "Call", Prompt: "x", Session: session, At: types.Ptr(time.Now().Add(-time.Hour))})
	assert.Error(t, err, "a time which has passed")
	_, err = c.CreateHeartbeat(ctx, schema.HeartbeatMeta{Title: "Call", Prompt: "x", Session: session, Schedule: "0 25 * * *"})
	assert.Error(t, err, "an invalid schedule")

	// Listed, and for the session
	list, err := c.ListHeartbeats(ctx, schema.HeartbeatListRequest{})
	require.NoError(t, err)
	assert.Equal(t, uint(2), list.Count)
	list, err = c.ListHeartbeats(ctx, schema.HeartbeatListRequest{Session: session.String()})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, daily.ID, list.Body[0].ID)

	// Got, changed, and deleted
	got, err := c.GetHeartbeat(ctx, once.ID)
	require.NoError(t, err)
	assert.Equal(t, "Call", got.Title)
	got, err = c.UpdateHeartbeat(ctx, once.ID, schema.HeartbeatRequest{Title: types.Ptr("Call Mum"), Schedule: types.Ptr("0 18 * * *")})
	require.NoError(t, err)
	assert.Equal(t, "Call Mum", got.Title)
	assert.Equal(t, "0 18 * * *", got.Schedule)
	assert.Nil(t, got.At)
	_, err = c.DeleteHeartbeat(ctx, once.ID)
	require.NoError(t, err)
	_, err = c.GetHeartbeat(ctx, once.ID)
	assert.Error(t, err)
	_, err = c.DeleteHeartbeat(ctx, once.ID)
	assert.Error(t, err)
}
