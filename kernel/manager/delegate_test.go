package manager

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
)

type mockDelegateConnector struct{}

func (*mockDelegateConnector) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (*mockDelegateConnector) ListTools(context.Context) ([]llm.Tool, error) {
	return nil, nil
}

func (*mockDelegateConnector) ListPrompts(context.Context) ([]llm.Prompt, error) {
	return nil, nil
}

func (*mockDelegateConnector) ListResources(context.Context) ([]llm.Resource, error) {
	return nil, nil
}

func TestWithConnectorRequiresIdentifier(t *testing.T) {
	var opts manageropt
	opts.defaults("test", "1.0")

	if err := WithConnector("bad name", &mockDelegateConnector{})(&opts); err == nil {
		t.Fatal("expected invalid identifier error")
	}
}

func TestDelegateCreateConnectorLocal(t *testing.T) {
	local := &mockDelegateConnector{}
	delegate := NewDelegate("test", "1.0", map[string]llm.Connector{"memory": local})

	var events []toolkit.ConnectorEvent
	conn, err := delegate.CreateConnector("memory", func(evt toolkit.ConnectorEvent) {
		events = append(events, evt)
	})
	if err != nil {
		t.Fatal(err)
	}
	if conn != local {
		t.Fatal("expected local connector to be returned")
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Kind != toolkit.ConnectorEventStateChange {
		t.Fatalf("expected state change event, got %v", events[0].Kind)
	}
}
