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

	// ConnectorEventDisconnected is fired when a connector stops running.
	// The Err field is populated for unexpected disconnects.
	ConnectorEventDisconnected

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
	// Set for connector-originated events (excluding ConnectorEventStateChange,
	// which is handled internally and never reaches the delegate); nil for
	// builtin add/remove events.
	Connector llm.Connector

	// State is populated for ConnectorEventStateChange events.
	State schema.ConnectorState

	// Err is populated for ConnectorEventDisconnected events when the connector
	// stopped due to an unexpected error.
	Err error

	// URI is populated for ConnectorEventResourceUpdated events.
	URI string
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (e ConnectorEventKind) String() string {
	switch e {
	case ConnectorEventStateChange:
		return "Connected"
	case ConnectorEventDisconnected:
		return "Disconnected"
	case ConnectorEventToolListChanged:
		return "ToolListChanged"
	case ConnectorEventPromptListChanged:
		return "PromptListChanged"
	case ConnectorEventResourceListChanged:
		return "ResourceListChanged"
	case ConnectorEventResourceUpdated:
		return "ResourceUpdated"
	default:
		return "Unknown"
	}
}

///////////////////////////////////////////////////////////////////////////////
// HELPER FUNCTIONS

// StateChangeEvent returns a ConnectorEventStateChange event with the given state.
func StateChangeEvent(state schema.ConnectorState) ConnectorEvent {
	return ConnectorEvent{Kind: ConnectorEventStateChange, State: state}
}

// DisconnectedEvent returns a ConnectorEventDisconnected event with the given error.
func DisconnectedEvent(err error) ConnectorEvent {
	return ConnectorEvent{Kind: ConnectorEventDisconnected, Err: err}
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
