package schema

import (
	"encoding/json"
	"fmt"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// The result of generating a message (stopped, error, etc.)
type ResultType string

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	ResultStop          ResultType = "stop"           // Normal completion
	ResultMaxTokens     ResultType = "max_tokens"     // Truncated due to max tokens
	ResultBlocked       ResultType = "blocked"        // Blocked by safety, recitation, or content filter
	ResultToolCall      ResultType = "tool_call"      // Model requested a tool call
	ResultError         ResultType = "error"          // Generation error
	ResultOther         ResultType = "other"          // Other/unknown finish reason
	ResultMaxIterations ResultType = "max_iterations" // Tool-calling loop exhausted max iterations
)

// ResultOK is an alias for ResultStop (normal completion).
const ResultOK = ResultStop

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r ResultType) String() string {
	return string(r)
}

////////////////////////////////////////////////////////////////////////////////
// JSONIFY

// UnmarshalJSON decodes a result, which must be one of the results
func (r *ResultType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch result := ResultType(s); result {
	case ResultStop, ResultMaxTokens, ResultBlocked, ResultToolCall, ResultError, ResultOther, ResultMaxIterations:
		*r = result
	default:
		return fmt.Errorf("unknown result type: %q", s)
	}
	return nil
}
