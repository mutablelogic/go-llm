package registry_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"uuid"

	// Packages
	agents "github.com/mutablelogic/go-llm/agents"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func Test_Traced(t *testing.T) {
	// What's said, such as an agent's input, isn't in any span, though its
	// length is, and the rest of the request
	const said = "s3cret"
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	r, err := registry.New(registry.WithTracer(tracer), registry.WithToolkit(test.NewToolkit(t)), registry.WithAgentStore(test.NewAgentStore()), registry.WithSessionStore(test.NewSessionStore()), registry.WithEventStore(test.NewEventStore()))
	require.NoError(t, err)
	reply := call(toolkit.RespondName, `{"result":{"message":"`+said+`","importance":"notify"}}`)
	p := newScripted(reply, reply, reply, text(said))
	require.NoError(t, r.Add("", p))
	all, err := agents.All()
	require.NoError(t, err)
	for _, meta := range all {
		_, _, err := r.CreateAgent(ctx, meta)
		require.NoError(t, err)
	}
	parent, err := r.CreateSession(ctx, schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("model")}})
	require.NoError(t, err)
	input := json.RawMessage(`{"title":"x","prompt":"` + said + `","data":"` + said + `","context":"Now"}`)

	// An agent, an event, one which is triggered, and a question
	_, err = r.RunAgent(ctx, registry.DefaultEventAgent, schema.RunAgentRequest{Input: input, Session: parent.ID}, nil)
	require.NoError(t, err)
	_, err = r.RunEvent(ctx, uuid.New(), schema.RunEventRequest{Session: parent.ID, Title: "An event", Input: input}, nil)
	require.NoError(t, err)
	triggered, err := r.TriggerEvent(ctx, schema.EventRequest{Session: parent.ID, Source: "inbox", Title: "Triggered", Prompt: said, Data: json.RawMessage(`{"url":"` + said + `"}`)})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		event, err := r.GetEvent(ctx, triggered.Event.ID)
		return err == nil && event.Status != schema.EventScheduled
	}, 5*time.Second, 10*time.Millisecond)
	req := question(said)
	_, err = r.Ask(ctx, req, nil)
	require.NoError(t, err)

	// They were asked, and the input was theirs
	p.mu.Lock()
	require.Len(t, p.asked, 4)
	for _, asked := range p.asked {
		assert.Contains(t, asked.messages[0].Text(), said)
	}
	p.mu.Unlock()

	attrs := map[string]map[string]string{}
	for _, span := range recorder.Ended() {
		values := map[string]string{}
		for _, kv := range span.Attributes() {
			values[string(kv.Key)] = kv.Value.Emit()
			assert.NotContains(t, kv.Value.Emit(), said, "%s %s", span.Name(), kv.Key)
		}
		for _, event := range span.Events() {
			for _, kv := range event.Attributes {
				assert.NotContains(t, kv.Value.Emit(), said, "%s %s %s", span.Name(), event.Name, kv.Key)
			}
		}
		attrs[span.Name()] = values
	}
	for name, key := range map[string]string{
		"llm.registry.RunAgent":     "arg.req.input",
		"llm.registry.RunEvent":     "arg.req.input",
		"llm.registry.TriggerEvent": "arg.req.data",
		"llm.registry.Ask":          "arg.req.messages",
	} {
		require.Contains(t, attrs, name)
		assert.NotEmpty(t, attrs[name][key], name)
		assert.NotEqual(t, "0", attrs[name][key], name)
	}
	assert.Contains(t, attrs["llm.registry.RunEvent"]["arg.req"], parent.ID.String())
	assert.Contains(t, attrs["llm.registry.RunAgent"]["arg.req"], "session")
	assert.Contains(t, attrs["llm.registry.TriggerEvent"]["arg.req"], "inbox")
	assert.Equal(t, "6", attrs["llm.registry.TriggerEvent"]["arg.req.prompt"])
	assert.Len(t, req.Messages, 1, "the request is as it was")
}
