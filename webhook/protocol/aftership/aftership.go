// Package aftership is AfterShip Tracking's webhook protocol: its calls are
// signed, with the account's webhook secret, in aftership-hmac-sha256, and
// say what happened, and their delivery, in the body's event, and event_id
//
// https://www.aftership.com/docs/tracking/webhook/webhook-signature
package aftership

import (
	"crypto/hmac"
	"crypto/sha256"
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
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	if !common.Equal(got, mac.Sum(nil)) {
		return llm.ErrForbidden.Withf("the call's %s isn't the body's, with the webhook's secret", headerSignature)
	}
	return nil
}

// Identify returns the body's event, such as tracking_update, and its
// event_id
func (Protocol) Identify(_ http.Header, values json.RawMessage) (string, string) {
	return common.Field(values, "event"), common.Field(values, "event_id")
}
