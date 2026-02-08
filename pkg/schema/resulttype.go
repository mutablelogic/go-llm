package schema

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
