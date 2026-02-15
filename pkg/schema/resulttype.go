package schema

import (
	"encoding/json"
	"fmt"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// The result of generating a message (stopped, error, etc.)
type ResultType uint

////////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	ResultStop      ResultType = iota // Normal completion
	ResultMaxTokens                   // Truncated due to max tokens
	ResultBlocked                     // Blocked by safety, recitation, or content filter
	ResultToolCall                    // Model requested a tool call
	ResultError                       // Generation error
	ResultOther                       // Other/unknown finish reason
)

// ResultOK is an alias for ResultStop (normal completion).
const ResultOK = ResultStop

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r ResultType) String() string {
	switch r {
	case ResultStop:
		return "stop"
	case ResultMaxTokens:
		return "max_tokens"
	case ResultBlocked:
		return "blocked"
	case ResultToolCall:
		return "tool_call"
	case ResultError:
		return "error"
	case ResultOther:
		return "other"
	default:
		return "unknown"
	}
}

////////////////////////////////////////////////////////////////////////////////
// JSON MARSHAL

func (r ResultType) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

func (r *ResultType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "stop":
		*r = ResultStop
	case "max_tokens":
		*r = ResultMaxTokens
	case "blocked":
		*r = ResultBlocked
	case "tool_call":
		*r = ResultToolCall
	case "error":
		*r = ResultError
	case "other":
		*r = ResultOther
	default:
		return fmt.Errorf("unknown result type: %q", s)
	}
	return nil
}
