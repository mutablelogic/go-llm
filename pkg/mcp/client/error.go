package httpclient

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"

	// Packages
	"github.com/mutablelogic/go-server/pkg/httpresponse"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// UnauthorizedError is returned by connect when the server replies with 401.
// It wraps httpresponse.ErrNotAuthorized and carries the parsed fields from
// the Www-Authenticate response header as a map of key→value pairs.
type UnauthorizedError struct {
	fields map[string]string
}

var reField = regexp.MustCompile(`(\w+)="([^"]*)"`)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewUnauthorizedError parses the Www-Authenticate values from header into an
// UnauthorizedError.
func NewUnauthorizedError(header http.Header) *UnauthorizedError {
	e := &UnauthorizedError{
		fields: make(map[string]string),
	}
	for _, v := range header.Values("Www-Authenticate") {
		for _, m := range reField.FindAllStringSubmatch(v, -1) {
			e.fields[m[1]] = m[2]
		}
	}
	return e
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Error implements the error interface. It returns the error_description if
// present, falling back to the error code, then a generic message.
func (e *UnauthorizedError) Error() string {
	code := e.fields["error"]
	desc := e.fields["error_description"]
	switch {
	case desc != "" && code != "":
		return fmt.Sprintf("%s: %s", code, desc)
	case desc != "":
		return desc
	case code != "":
		return code
	default:
		return "unauthorized"
	}
}

// Unwrap returns httpresponse.ErrNotAuthorized so that errors.Is checks work.
func (e *UnauthorizedError) Unwrap() error {
	return httpresponse.ErrNotAuthorized
}

// ResourceMetadata returns the resource_metadata URL from the Www-Authenticate
// header (RFC 9728), or an empty string if absent.
func (e *UnauthorizedError) ResourceMetadata() string {
	return e.fields["resource_metadata"]
}

// Get returns the value of an arbitrary field from the Www-Authenticate header,
// or an empty string if the field is absent.
func (e *UnauthorizedError) Get(key string) string {
	return e.fields[key]
}

// Keys returns the field names present in the Www-Authenticate header.
func (e *UnauthorizedError) Keys() []string {
	keys := make([]string, 0, len(e.fields))
	for k := range e.fields {
		keys = append(keys, k)
	}
	return keys
}

// ErrNotConnected is returned by methods that require an active session when
// no session has been established yet.
var ErrNotConnected = errors.New("not connected")

// IsUnauthorized reports whether err is (or wraps) an UnauthorizedError / 401.
func IsUnauthorized(err error) bool {
	return errors.Is(err, httpresponse.ErrNotAuthorized)
}

// AsUnauthorized returns the *UnauthorizedError inside err, or nil.
func AsUnauthorized(err error) *UnauthorizedError {
	var e *UnauthorizedError
	if errors.As(err, &e) {
		return e
	}
	return nil
}
