package llm

import (
	"fmt"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	ErrSuccess Err = iota
	ErrNotFound
	ErrBadParameter
	ErrNotImplemented
	ErrConflict
	ErrInternalServerError
	ErrMaxTokens
	ErrRefusal
	ErrPauseTurn
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Errors
type Err int

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (e Err) Error() string {
	switch e {
	case ErrSuccess:
		return "success"
	case ErrNotFound:
		return "not found"
	case ErrBadParameter:
		return "bad parameter"
	case ErrNotImplemented:
		return "not implemented"
	case ErrConflict:
		return "conflict"
	case ErrInternalServerError:
		return "internal server error"
	case ErrMaxTokens:
		return "response truncated: max tokens reached"
	case ErrRefusal:
		return "model refused to respond"
	case ErrPauseTurn:
		return "model paused, continuation required"
	}
	return fmt.Sprintf("error code %d", int(e))
}

func (e Err) With(args ...interface{}) error {
	return fmt.Errorf("%w: %s", e, fmt.Sprint(args...))
}

func (e Err) Withf(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", e, fmt.Sprintf(format, args...))
}
