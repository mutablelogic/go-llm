// Package github is GitHub's webhook protocol: its calls are signed, with
// the secret, in X-Hub-Signature-256, say what happened in X-GitHub-Event,
// and the body's action, and their delivery in X-GitHub-Delivery. A call
// whose content type is a form has its JSON in the form's payload field.
//
// https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
package github

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	common "github.com/mutablelogic/go-llm/webhook/protocol/common"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Protocol is GitHub's protocol
type Protocol struct{}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Name is the protocol's name
	Name = "github"

	headerEvent     = "X-GitHub-Event"
	headerDelivery  = "X-GitHub-Delivery"
	headerHook      = "X-GitHub-Hook-ID"
	headerSignature = "X-Hub-Signature-256"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the protocol
func New() Protocol { return Protocol{} }

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (Protocol) Name() string { return Name }

func (Protocol) Headers() []string {
	return []string{headerEvent, headerDelivery, headerHook, headerSignature}
}

func (Protocol) Handshake(*http.Request, string) ([]byte, bool) { return nil, false }

// Check returns nil when the call is a POST, with its event in
// X-GitHub-Event, as every call of GitHub's is
func (Protocol) Check(method string, header http.Header) error {
	if method != http.MethodPost {
		return llm.ErrBadParameter.Withf("the call is a %s, and GitHub's are a POST", method)
	}
	if header.Get(headerEvent) == "" {
		return llm.ErrBadParameter.Withf("the call has no %s, which GitHub's have", headerEvent)
	}
	return nil
}

// Decode returns the body, or, when it's a form, its payload field, which is
// the JSON
func (Protocol) Decode(header http.Header, body []byte) (json.RawMessage, error) {
	if common.MediaType(header) == "application/x-www-form-urlencoded" {
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("body isn't a valid form: %v", err)
		}
		if payload := form.Get("payload"); payload != "" {
			if !json.Valid([]byte(payload)) {
				return nil, llm.ErrBadParameter.With("the form's payload isn't valid JSON")
			}
			return json.RawMessage(payload), nil
		}
	}
	return common.Decode(header, body)
}

// Verify returns nil when X-Hub-Signature-256 is sha256=, and the body's
// HMAC-SHA256, with the secret, in hex
func (Protocol) Verify(header http.Header, body []byte, secret string) error {
	if secret == "" {
		return common.NoSecret()
	}
	signature, ok := strings.CutPrefix(header.Get(headerSignature), "sha256=")
	if !ok {
		return llm.ErrForbidden.Withf("the call has no %s", headerSignature)
	}
	got, err := hex.DecodeString(signature)
	if err != nil {
		return llm.ErrForbidden.Withf("the call's %s isn't hex", headerSignature)
	}
	if !common.Equal(got, common.HMAC(secret, body)) {
		return llm.ErrForbidden.Withf("the call's %s isn't the body's, with the webhook's secret", headerSignature)
	}
	return nil
}

// Sample returns a ping, which GitHub sends when a webhook is set up
func (Protocol) Sample(delivery string) (http.Header, []byte) {
	header, body := common.Sample(nil)
	header.Set(headerEvent, "ping")
	header.Set(headerDelivery, delivery)
	return header, body
}

// Sign sets X-Hub-Signature-256
func (Protocol) Sign(header http.Header, body []byte, secret string) {
	header.Set(headerSignature, "sha256="+hex.EncodeToString(common.HMAC(secret, body)))
}

// Identify returns X-GitHub-Event, and the body's action, when it has one,
// such as issues.opened, and X-GitHub-Delivery
func (Protocol) Identify(header http.Header, values json.RawMessage) (string, string) {
	event := header.Get(headerEvent)
	if action := common.Field(values, "action"); event != "" && action != "" {
		event += "." + action
	}
	return event, header.Get(headerDelivery)
}
