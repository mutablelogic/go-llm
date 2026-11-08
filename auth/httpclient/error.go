package httpclient

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AuthError is a server's challenge, as a request to it isn't authorized:
// its scheme, such as Bearer, and its parameters, such as its realm, and
// resource_metadata, the URL of the resource's metadata
type AuthError struct {
	Scheme string     `json:"scheme"`
	Params url.Values `json:"params,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// AuthenticateHeader is the header of a server's challenge
const AuthenticateHeader = "WWW-Authenticate"

var (
	// The scheme of a challenge, and the rest of it, and each of its
	// parameters, whose values are quoted, or not
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/WWW-Authenticate
	reChallenge = regexp.MustCompile(`^\s*([^\s,]+)\s*(.*)$`)
	reParam     = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_-]*)\s*=\s*(?:"((?:\\.|[^"\\])*)"|([^\s,"]+))`)
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// ParseChallenge returns the challenge of the header, which is its first,
// or nil, when it hasn't one
func ParseChallenge(header http.Header) *AuthError {
	value := strings.TrimSpace(header.Get(AuthenticateHeader))
	match := reChallenge.FindStringSubmatch(value)
	if value == "" || match == nil {
		return nil
	}
	result := &AuthError{Scheme: match[1], Params: url.Values{}}
	for _, param := range reParam.FindAllStringSubmatch(match[2], -1) {
		value := param[3]
		if param[3] == "" {
			if unquoted, err := strconv.Unquote(`"` + param[2] + `"`); err == nil {
				value = unquoted
			} else {
				value = param[2]
			}
		}
		result.Params.Add(param[1], value)
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AsAuthError returns the AuthError the error is, or wraps, or nil
func AsAuthError(err error) *AuthError {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr
	}
	return nil
}

// Get returns the value of the challenge's parameter, or nothing
func (e *AuthError) Get(key string) string {
	return e.Params.Get(key)
}

// Error returns the challenge's error, and its description, when it has
// them, or that it isn't authorized
func (e *AuthError) Error() string {
	code, description := e.Get("error"), e.Get("error_description")
	switch {
	case code != "" && description != "":
		return fmt.Sprintf("unauthorized: %s: %s", code, description)
	case description != "":
		return "unauthorized: " + description
	case code != "":
		return "unauthorized: " + code
	}
	return "unauthorized"
}
