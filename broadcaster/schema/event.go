package schema

import (
	"encoding/json"
	"fmt"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Event is a change: how it changed, which is its action, what changed,
// which is a table of a schema, and which row, or rows, changed, when it
// says, such as by an identifier which the rows share, and what the table
// says of the change, such as a row's new serial, when it says anything
type Event struct {
	Action EventAction     `json:"action" help:"How it changed, which is create, update or delete" example:"create"`
	Schema string          `json:"schema,omitempty" help:"Schema of the table which changed" example:"llm"`
	Table  string          `json:"table,omitempty" help:"Table which changed" example:"provider"`
	Row    string          `json:"row,omitempty" help:"Row, or rows, which changed, such as by an identifier" example:"ollama"`
	Data   json.RawMessage `json:"data,omitempty" help:"What the table says of the change, such as a row's new serial"`
}

// EventAction is how something changed: it was created, updated or deleted
type EventAction string

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The events of a stream of the broadcaster's events: each event, and that
// the server is stopping, before the stream ends, so a client knows it
// ended as there will be no more events
const (
	EventName     = "event"
	EventShutdown = "shutdown"
)

const (
	EventCreate EventAction = "create" // Created
	EventUpdate EventAction = "update" // Updated
	EventDelete EventAction = "delete" // Deleted
)

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (e Event) String() string {
	return types.Stringify(e)
}

func (a EventAction) String() string {
	return string(a)
}

////////////////////////////////////////////////////////////////////////////////
// JSONIFY

// UnmarshalJSON decodes an action, which must be one of the actions
func (a *EventAction) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch action := EventAction(s); action {
	case EventCreate, EventUpdate, EventDelete:
		*a = action
	default:
		return fmt.Errorf("unknown event action: %q", s)
	}
	return nil
}
