package httpclient_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	registry "github.com/mutablelogic/go-llm/registry"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
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

	// An event which is triggered is scheduled, and fails in the
	// background, as there's no agent to run it, and one of its source isn't
	// scheduled again, until then
	parent, err := client.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(eliza.ModelName)}})
	require.NoError(t, err)
	triggered, err := client.TriggerEvent(ctx, schema.EventRequest{Session: parent.ID, Source: "inbox", Prompt: "Summarise it", Data: json.RawMessage(`{"path":"/inbox/a.pdf"}`)})
	require.NoError(t, err)
	assert.True(t, triggered.Scheduled)
	assert.Equal(t, schema.EventScheduled, triggered.Event.Status)
	require.Eventually(t, func() bool {
		got, err := client.GetEvent(ctx, triggered.Event.ID)
		return err == nil && got.Status == schema.EventFailed
	}, 5*time.Second, 10*time.Millisecond)
	_, err = client.TriggerEvent(ctx, schema.EventRequest{Session: uuid.New(), Prompt: "x"})
	assert.Error(t, err, "a session which doesn't exist")

	// Got, and deleted
	got, err := client.GetEvent(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Call your mother", got.Message)
	_, err = client.DeleteEvent(ctx, created.ID)
	require.NoError(t, err)
	_, err = client.GetEvent(ctx, created.ID)
	assert.Error(t, err)
}
