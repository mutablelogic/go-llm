// Package protocol is how services call webhooks: the Protocol interface,
// and the factory of the protocols, each of which is a package of its own,
// in this folder, such as github. A new protocol is a package, which is
// added to protocols.
package protocol

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	aftership "github.com/mutablelogic/go-llm/webhook/protocol/aftership"
	generic "github.com/mutablelogic/go-llm/webhook/protocol/generic"
	github "github.com/mutablelogic/go-llm/webhook/protocol/github"
	pidgeon "github.com/mutablelogic/go-llm/webhook/protocol/pidgeon"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Protocol is how a service calls its webhooks: which of a call's headers
// say what it is, how its body is JSON values, how it's verified, with the
// webhook's secret, and what it says happened, such as GitHub's, whose calls
// are signed with X-Hub-Signature-256, and say their event, and delivery, in
// X-GitHub-Event, and X-GitHub-Delivery
type Protocol interface {
	// Name returns the protocol's name, which is a webhook's type, such as
	// github
	Name() string

	// Headers returns the canonical names of the headers which are kept,
	// besides Content-Type, and User-Agent, such as the event's, and the
	// signature's
	Headers() []string

	// Handshake returns the response to a request which checks the webhook,
	// such as a GET, which echoes a challenge, as it's registered, with the
	// secret, and true, or false, when it isn't one, which is a call
	Handshake(r *http.Request, secret string) ([]byte, bool)

	// Check returns nil when a call, of the method, and with the headers, is
	// one which the service makes, such as a POST, with its event, and
	// ErrBadParameter otherwise. It's whether a call looks like the service's,
	// which anyone can make one look like, rather than whether it's verified.
	Check(method string, header http.Header) error

	// Decode returns the body, with its headers, as JSON values, or nil,
	// when it has none, or isn't text
	Decode(header http.Header, body []byte) (json.RawMessage, error)

	// Verify returns nil when the body was signed with the secret, which
	// isn't empty, and ErrForbidden otherwise
	Verify(header http.Header, body []byte, secret string) error

	// Sample returns the headers, and the body, of a call which the service
	// could make, with the identifier of its delivery, such as for a test. It
	// isn't signed.
	Sample(delivery string) (http.Header, []byte)

	// Sign signs the body with the secret, as the service does, in the
	// headers, so it's verified with it
	Sign(header http.Header, body []byte, secret string)

	// Identify returns what happened, such as issues.opened, and the
	// identifier of the delivery, which a delivery which is sent again has
	// too, or empty, when they aren't known
	Identify(header http.Header, values json.RawMessage) (event, delivery string)
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Default is the protocol of a webhook which has no type
const Default = generic.Name

// protocols are the protocols, by their names, which a new one is added to
var protocols = map[string]Protocol{
	generic.Name:   generic.New(),
	github.Name:    github.New(),
	aftership.Name: aftership.New(),
	pidgeon.Name:   pidgeon.New(),
}

// commonHeaders are the headers which are kept, of every call
var commonHeaders = []string{"Content-Type", "User-Agent"}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Names returns the names of the protocols, in order
func Names() []string {
	names := make([]string, 0, len(protocols))
	for name := range protocols {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// New returns the protocol with the name, a webhook's type, which is the
// default, generic, when it's empty, or ErrBadParameter, when there's none
func New(name string) (Protocol, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = Default
	}
	if p, ok := protocols[name]; ok {
		return p, nil
	}
	return nil, llm.ErrBadParameter.Withf("webhook type %q isn't one of %s", name, strings.Join(Names(), ", "))
}

// KeptHeaders returns the headers of the call which the protocol keeps, by
// their canonical names, with their first values, or nil, when there are
// none
func KeptHeaders(p Protocol, header http.Header) map[string]string {
	kept := make(map[string]string)
	for _, name := range append(slices.Clone(commonHeaders), p.Headers()...) {
		if value := header.Get(name); value != "" {
			kept[http.CanonicalHeaderKey(name)] = value
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}
