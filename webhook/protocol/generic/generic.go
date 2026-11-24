// Package generic is the webhook protocol of a service which has none of
// its own: its calls are verified by the secret, as a bearer token, when the
// service can send one, and say nothing of what happened
package generic

import (
	"encoding/json"
	"net/http"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	common "github.com/mutablelogic/go-llm/webhook/protocol/common"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Protocol is the generic protocol
type Protocol struct{}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the protocol's name
const Name = "generic"

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the protocol
func New() Protocol { return Protocol{} }

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (Protocol) Name() string { return Name }

func (Protocol) Headers() []string { return nil }

func (Protocol) Handshake(*http.Request, string) ([]byte, bool) { return nil, false }

func (Protocol) Decode(header http.Header, body []byte) (json.RawMessage, error) {
	return common.Decode(header, body)
}

// Verify returns nil when the call has the secret as a bearer token, in its
// Authorization header, whose scheme is of any case
func (Protocol) Verify(header http.Header, _ []byte, secret string) error {
	if secret == "" {
		return common.NoSecret()
	}
	scheme, token, ok := strings.Cut(strings.TrimSpace(header.Get("Authorization")), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || !common.EqualString(strings.TrimSpace(token), secret) {
		return llm.ErrForbidden.With("the call's bearer token isn't the webhook's secret")
	}
	return nil
}

func (Protocol) Identify(http.Header, json.RawMessage) (string, string) { return "", "" }
