package httpclient_test

import (
	"context"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	registry "github.com/mutablelogic/go-llm/registry"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Events(t *testing.T) {
	ctx := context.Background()
	events := test.NewEventStore()
	provider, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{provider}, registry.WithSessionStore(test.NewSessionStore()), registry.WithEventStore(events)))
	client, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)

	// An event's outcome, which the registry stores, as it runs it
	session := uuid.New()
	created, err := events.Create(ctx, schema.EventMeta{Run: uuid.New(), Session: session, Title: "Call", Message: "Call your mother", Importance: schema.ImportanceAlert, Status: schema.EventPending})
	require.NoError(t, err)

	// Listed, and by session, and status
	list, err := client.ListEvents(ctx, schema.EventListRequest{})
	require.NoError(t, err)
	require.Equal(t, uint(1), list.Count)
	assert.Equal(t, created.ID, list.Body[0].ID)
	list, err = client.ListEvents(ctx, schema.EventListRequest{Session: session.String(), Status: schema.EventPending})
	require.NoError(t, err)
	assert.Equal(t, uint(1), list.Count)
	list, err = client.ListEvents(ctx, schema.EventListRequest{Status: schema.EventDelivered})
	require.NoError(t, err)
	assert.Zero(t, list.Count)
	_, err = client.ListEvents(ctx, schema.EventListRequest{Session: "x"})
	assert.Error(t, err)

	// Got, and deleted
	got, err := client.GetEvent(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Call your mother", got.Message)
	_, err = client.DeleteEvent(ctx, created.ID)
	require.NoError(t, err)
	_, err = client.GetEvent(ctx, created.ID)
	assert.Error(t, err)
}
