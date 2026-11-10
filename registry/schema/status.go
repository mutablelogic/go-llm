package schema

import (
	"encoding/json"
	"fmt"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SessionStatus is who's doing what in a session: someone is typing a
// message, its model is replying to one, or thinking, as it replies, or
// neither, when it's idle
type SessionStatus string

// SessionStatusRequest sets a session's status, which a client sets to
// typing, as someone types a message, or idle, when they stop
type SessionStatusRequest struct {
	Status SessionStatus `json:"status" help:"Status of the session, which is typing or idle" enum:"typing,idle" example:"typing"`
}

// SessionEvent is what an event of a session says of the change: the
// session's new serial, when it was stored, or its new status, when that
// changed
type SessionEvent struct {
	Serial uint64        `json:"serial,omitempty" help:"Serial of the session, as it was stored" example:"3"`
	Status SessionStatus `json:"status,omitempty" help:"Status of the session, as it changed" example:"replying"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	SessionIdle     SessionStatus = "idle"     // Neither typing nor replying
	SessionTyping   SessionStatus = "typing"   // Someone is typing a message
	SessionReplying SessionStatus = "replying" // The model is replying
	SessionThinking SessionStatus = "thinking" // The model is thinking, or its tools are running, as it replies
)

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (s SessionStatus) String() string {
	return string(s)
}

func (r SessionStatusRequest) String() string {
	return types.Stringify(r)
}

func (e SessionEvent) String() string {
	return types.Stringify(e)
}

///////////////////////////////////////////////////////////////////////////////
// JSONIFY

// UnmarshalJSON decodes a status, which must be one of the statuses
func (s *SessionStatus) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch status := SessionStatus(value); status {
	case SessionIdle, SessionTyping, SessionReplying, SessionThinking:
		*s = status
	default:
		return fmt.Errorf("unknown session status: %q", value)
	}
	return nil
}
