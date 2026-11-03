package llm

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	// Packages
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Errors
type Err int

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	ErrSuccess Err = iota
	ErrNotFound
	ErrBadParameter
	ErrNotImplemented
	ErrConflict
	ErrInternalServerError
	ErrServiceUnavailable
	ErrForbidden
	ErrMaxTokens // the model's reply was truncated, as it reached the most tokens
	ErrRefusal   // the model refused to reply
	ErrPauseTurn // the model paused its turn, which is to be continued
)

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
	case ErrServiceUnavailable:
		return "service unavailable"
	case ErrForbidden:
		return "forbidden"
	case ErrMaxTokens:
		return "response truncated: max tokens reached"
	case ErrRefusal:
		return "model refused to respond"
	case ErrPauseTurn:
		return "model paused, continuation required"
	}
	return fmt.Sprintf("error code %d", int(e))
}

func (e Err) With(args ...any) error {
	return fmt.Errorf("%w: %s", e, fmt.Sprint(args...))
}

// Withf returns an error wrapping the error code, with a formatted message.
// As with fmt.Errorf, the format may use %w to wrap errors, so that they can
// still be matched with errors.Is and errors.As.
func (e Err) Withf(format string, args ...any) error {
	return fmt.Errorf("%w: %w", e, fmt.Errorf(format, args...))
}

func (e Err) HTTP() httpresponse.Err {
	switch e {
	case ErrNotFound:
		return httpresponse.ErrNotFound
	case ErrBadParameter, ErrMaxTokens, ErrRefusal:
		return httpresponse.ErrBadRequest
	case ErrConflict:
		return httpresponse.ErrConflict
	case ErrNotImplemented:
		return httpresponse.ErrNotImplemented
	case ErrInternalServerError:
		return httpresponse.ErrInternalError
	case ErrServiceUnavailable:
		return httpresponse.ErrServiceUnavailable
	case ErrForbidden:
		return httpresponse.ErrForbidden
	default:
		return httpresponse.ErrInternalError
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// HTTPErr translates an error into an error wrapping a HTTP status code,
// which httpresponse.Error uses for the response. The message is prefixed
// with the status text, and errors.Is and errors.As still match the original
// error.
//
// An llm.Err is translated with its HTTP method, even when it wraps a HTTP
// status code, such as a provider's response, and fs errors (fs.ErrNotExist
// and so on) to the corresponding status code. Other errors which already
// wrap a HTTP status code are returned unchanged, and any other error is an
// internal server error. Returns nil when err is nil.
func HTTPErr(err error) error {
	if err == nil {
		return nil
	}

	// An llm error, which is the error's status, rather than an error it
	// wraps, such as a provider's response
	var e Err
	if errors.As(err, &e) {
		return e.HTTP().Withf("%w", err)
	}

	// Already a HTTP error
	var code httpresponse.Err
	if errors.As(err, &code) {
		return err
	}

	// A request whose body ends before it should, such as an upload which
	// is interrupted, is a bad request
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return httpresponse.ErrBadRequest.Withf("%w", err)
	}

	// A filesystem error
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return httpresponse.ErrNotFound.Withf("%w", err)
	case errors.Is(err, fs.ErrExist):
		return httpresponse.ErrConflict.Withf("%w", err)
	case errors.Is(err, fs.ErrPermission):
		return httpresponse.ErrForbidden.Withf("%w", err)
	case errors.Is(err, fs.ErrInvalid):
		return httpresponse.ErrBadRequest.Withf("%w", err)
	case errors.Is(err, fs.ErrClosed):
		return httpresponse.ErrServiceUnavailable.Withf("%w", err)
	}

	// Any other error
	return httpresponse.ErrInternalError.Withf("%w", err)
}
