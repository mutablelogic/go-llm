// Package pidgeon is Pidgeon's webhook protocol: its calls are signed, with
// the endpoint's whsec_ secret, in Pidgeon-Signature, as
// t=<seconds>,v1=<hex HMAC-SHA256 of "<t>.<body>">, which is too old after
// five minutes, say what happened in Pidgeon-Event, and the body's type, and
// their delivery in the body's id
//
// https://pidgeon.ai/docs/webhooks
package pidgeon

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	common "github.com/mutablelogic/go-llm/webhook/protocol/common"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Protocol is Pidgeon's protocol
type Protocol struct{}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Name is the protocol's name
	Name = "pidgeon"

	headerEvent     = "Pidgeon-Event"
	headerSignature = "Pidgeon-Signature"

	// tolerance is how far from now a signature's time can be
	tolerance = 5 * time.Minute
)

// now returns the time, which tests change
var now = time.Now

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the protocol
func New() Protocol { return Protocol{} }

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (Protocol) Name() string { return Name }

func (Protocol) Headers() []string { return []string{headerEvent, headerSignature} }

func (Protocol) Handshake(*http.Request, string) ([]byte, bool) { return nil, false }

func (Protocol) Decode(header http.Header, body []byte) (json.RawMessage, error) {
	return common.Decode(header, body)
}

// Verify returns nil when one of Pidgeon-Signature's v1 signatures is the
// HMAC-SHA256 of its time, a dot, and the body, with the secret, in hex, and
// its time is within five minutes of now
func (Protocol) Verify(header http.Header, body []byte, secret string) error {
	if secret == "" {
		return common.NoSecret()
	}
	var timestamp string
	var signatures [][]byte
	for _, part := range strings.Split(header.Get(headerSignature), ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch key {
		case "t":
			timestamp = value
		case "v1":
			if signature, err := hex.DecodeString(value); err == nil {
				signatures = append(signatures, signature)
			}
		}
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || len(signatures) == 0 {
		return llm.ErrForbidden.Withf("the call has no %s, with a time, and a signature", headerSignature)
	}
	if age := now().Sub(time.Unix(seconds, 0)); age > tolerance || age < -tolerance {
		return llm.ErrForbidden.Withf("the call's %s is from %s, more than %s from now", headerSignature, time.Unix(seconds, 0).UTC().Format(time.RFC3339), tolerance)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		if common.Equal(signature, expected) {
			return nil
		}
	}
	return llm.ErrForbidden.Withf("the call's %s isn't the body's, with the webhook's secret", headerSignature)
}

// Identify returns Pidgeon-Event, or the body's type, such as
// message.received, and the body's id
func (Protocol) Identify(header http.Header, values json.RawMessage) (string, string) {
	event := header.Get(headerEvent)
	if event == "" {
		event = common.Field(values, "type")
	}
	return event, common.Field(values, "id")
}
