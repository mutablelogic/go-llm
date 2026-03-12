package toolkit

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ConnectorEventKind identifies the event type fired by a connector.
type ConnectorEventKind int

const (
	// ConnectorEventStateChange is fired after a successful connection handshake.
	// The State field of ConnectorEvent is populated.
	ConnectorEventStateChange ConnectorEventKind = iota

	// ConnectorEventToolListChanged is fired when the remote tool list changes.
	ConnectorEventToolListChanged

	// ConnectorEventPromptListChanged is fired when the remote prompt list changes.
	ConnectorEventPromptListChanged

	// ConnectorEventResourceListChanged is fired when the remote resource list changes.
	ConnectorEventResourceListChanged

	// ConnectorEventResourceUpdated is fired when a specific resource is updated.
	// The URI field of ConnectorEvent is populated.
	ConnectorEventResourceUpdated
)

// ConnectorEvent carries the event type and all relevant payload for a single
// notification delivered to a ToolkitDelegate via the onEvent callback.
type ConnectorEvent struct {
	// Kind identifies the type of event.
	Kind ConnectorEventKind

	// Connector is the toolkit's connector that fired the event.
	// Always set by the toolkit before the event reaches the delegate.
	Connector llm.Connector

	// State is populated for ConnectorEventStateChange events.
	State schema.ConnectorState

	// URI is populated for ConnectorEventResourceUpdated events.
	URI string
}

///////////////////////////////////////////////////////////////////////////////
// HELPER FUNCTIONS

// StateChangeEvent returns a ConnectorEventStateChange event with the given state.
func StateChangeEvent(state schema.ConnectorState) ConnectorEvent {
	return ConnectorEvent{Kind: ConnectorEventStateChange, State: state}
}

// ToolListChangeEvent returns a ConnectorEventToolListChanged event.
func ToolListChangeEvent() ConnectorEvent {
	return ConnectorEvent{Kind: ConnectorEventToolListChanged}
}

// PromptListChangeEvent returns a ConnectorEventPromptListChanged event.
func PromptListChangeEvent() ConnectorEvent {
	return ConnectorEvent{Kind: ConnectorEventPromptListChanged}
}

// ResourceListChangeEvent returns a ConnectorEventResourceListChanged event.
func ResourceListChangeEvent() ConnectorEvent {
	return ConnectorEvent{Kind: ConnectorEventResourceListChanged}
}

// ResourceUpdatedEvent returns a ConnectorEventResourceUpdated event with the given URI.
func ResourceUpdatedEvent(uri string) ConnectorEvent {
	return ConnectorEvent{Kind: ConnectorEventResourceUpdated, URI: uri}
}
