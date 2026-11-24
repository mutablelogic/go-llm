// Package aftership is AfterShip Tracking's webhook protocol: its calls are
// signed, with the account's webhook secret, in aftership-hmac-sha256, and
// say what happened, and their delivery, in the body's event, and event_id
//
// https://www.aftership.com/docs/tracking/webhook/webhook-signature
package aftership

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	common "github.com/mutablelogic/go-llm/webhook/protocol/common"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Protocol is AfterShip's protocol
type Protocol struct{}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Name is the protocol's name
	Name = "aftership"

	headerSignature = "Aftership-Hmac-Sha256"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the protocol
func New() Protocol { return Protocol{} }

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (Protocol) Name() string { return Name }

func (Protocol) Headers() []string { return []string{headerSignature} }

func (Protocol) Handshake(*http.Request, string) ([]byte, bool) { return nil, false }

// Check returns nil, as it isn't known which calls AfterShip doesn't make
func (Protocol) Check(string, http.Header) error { return nil }

func (Protocol) Decode(header http.Header, body []byte) (json.RawMessage, error) {
	return common.Decode(header, body)
}

// Verify returns nil when aftership-hmac-sha256 is the body's HMAC-SHA256,
// with the secret, in base64
func (Protocol) Verify(header http.Header, body []byte, secret string) error {
	if secret == "" {
		return common.NoSecret()
	}
	got, err := base64.StdEncoding.DecodeString(header.Get(headerSignature))
	if err != nil || len(got) == 0 {
		return llm.ErrForbidden.Withf("the call has no %s, in base64", headerSignature)
	}
	if !common.Equal(got, common.HMAC(secret, body)) {
		return llm.ErrForbidden.Withf("the call's %s isn't the body's, with the webhook's secret", headerSignature)
	}
	return nil
}

// Sample returns a call with an event
func (Protocol) Sample(delivery string) (http.Header, []byte) {
	return common.Sample(map[string]any{"event": "test", "event_id": delivery})
}

// Sign sets aftership-hmac-sha256
func (Protocol) Sign(header http.Header, body []byte, secret string) {
	header.Set(headerSignature, base64.StdEncoding.EncodeToString(common.HMAC(secret, body)))
}

// Identify returns the body's event, such as tracking_update, and its
// event_id
func (Protocol) Identify(_ http.Header, values json.RawMessage) (string, string) {
	return common.Field(values, "event"), common.Field(values, "event_id")
}
