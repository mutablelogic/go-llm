package schema

import (
	"errors"
	"fmt"

	// Packages
	pg "github.com/mutablelogic/go-pg"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
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
	ErrServiceUnavailable
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
	case ErrServiceUnavailable:
		return "service unavailable"
	}
	return fmt.Sprintf("error code %d", int(e))
}

func (e Err) HTTP() httpresponse.Err {
	switch e {
	case ErrNotFound:
		return httpresponse.ErrNotFound
	case ErrBadParameter:
		return httpresponse.ErrBadRequest
	case ErrConflict:
		return httpresponse.ErrConflict
	case ErrNotImplemented:
		return httpresponse.ErrNotImplemented
	case ErrInternalServerError:
		return httpresponse.ErrInternalError
	case ErrServiceUnavailable:
		return httpresponse.ErrServiceUnavailable
	case ErrMaxTokens, ErrRefusal:
		return httpresponse.ErrBadRequest
	default:
		return httpresponse.ErrInternalError
	}
}

func HTTPErr(err error) error {
	if err == nil {
		return nil
	}

	var httpErr httpresponse.Err
	if errors.As(err, &httpErr) {
		return err
	}

	var schemaErr Err
	if errors.As(err, &schemaErr) {
		return schemaErr.HTTP().With(err)
	}

	switch {
	case errors.Is(err, pg.ErrNotFound):
		return httpresponse.ErrNotFound.With(err)
	case errors.Is(err, pg.ErrBadParameter):
		return httpresponse.ErrBadRequest.With(err)
	case errors.Is(err, pg.ErrConflict):
		return httpresponse.ErrConflict.With(err)
	case errors.Is(err, pg.ErrNotImplemented):
		return httpresponse.ErrNotImplemented.With(err)
	case errors.Is(err, pg.ErrNotAvailable):
		return httpresponse.ErrServiceUnavailable.With(err)
	case errors.Is(err, pg.ErrDatabase):
		return httpresponse.ErrInternalError.With(err)
	}

	return httpresponse.ErrInternalError.With(err)
}

func (e Err) With(args ...any) error {
	return fmt.Errorf("%w: %s", e, fmt.Sprint(args...))
}

func (e Err) Withf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", e, fmt.Sprintf(format, args...))
}
