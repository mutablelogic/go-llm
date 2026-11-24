package schema

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Packages
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The statuses of calls, which they're listed by
const (
	CallAccepted = "accepted" // kept, as it was verified, when its webhook has a secret, and decoded
	CallRefused  = "refused"  // kept, with why it was refused
)

// How a test call is signed
const (
	SignatureGood = "good" // with the webhook's secret
	SignatureBad  = "bad"  // with a secret which isn't the webhook's
	SignatureNone = "none" // not signed
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Call is a call of a webhook, by a service, which is kept, as it was
// received: its method, its query, the headers which say what it is, such
// as its event, delivery and signature, its body, as JSON values, and its
// body, as it was sent, which a signature is checked against, and what its
// protocol says it is, whether it was verified, or is a test, and why it was
// refused, when it was
type Call struct {
	ID       uint64            `json:"id" readonly:"" help:"Serial number of the call"`
	Webhook  uint64            `json:"webhook" readonly:"" help:"Serial number of the webhook which was called"`
	Received time.Time         `json:"received" readonly:"" help:"When it was received"`
	Method   string            `json:"method" readonly:"" help:"Its HTTP method, such as POST, or GET" example:"POST"`
	Query    url.Values        `json:"query,omitempty" readonly:"" help:"Its query parameters"`
	Headers  map[string]string `json:"headers,omitempty" readonly:"" help:"The headers which say what it is, such as its content type, event, delivery and signature, by their canonical names" example:"{\"Content-Type\":\"application/json\",\"X-Github-Event\":\"issues\"}"`
	Values   json.RawMessage   `json:"values,omitempty" readonly:"" help:"Its body, as JSON: as it is, when it's JSON, an object of its fields, when it's a form, or else a string"`
	Event    string            `json:"event,omitempty" readonly:"" help:"What happened, which its protocol says, such as issues.opened" example:"issues.opened"`
	Delivery string            `json:"delivery,omitempty" readonly:"" help:"Identifier of its delivery, which its protocol says, which a delivery which is sent again has too"`
	Verified bool              `json:"verified" readonly:"" help:"Whether its signature was verified, with the webhook's secret"`
	Test     bool              `json:"test,omitempty" readonly:"" help:"Whether it's a test call, which no service sent"`
	Error    string            `json:"error,omitempty" readonly:"" help:"Why it was refused, such as that its signature isn't the webhook's, when it was, which is kept, so a secret which is wrong can be seen"`
	Body     []byte            `json:"body,omitempty" readonly:"" help:"Its body, as it was sent, up to a limit, which a signature is checked against"`
}

// CallListRequest is which calls are listed: those of the webhook, or the
// session's webhooks, when there's one, those which were accepted, or
// refused, or both, after the call with the serial number, and received
// since the time, oldest first, or newest, and at most the limit of them
type CallListRequest struct {
	Webhook uint64    `json:"webhook,omitempty" help:"Serial number of the webhook whose calls are listed" optional:""`
	Session string    `json:"session,omitempty" help:"Identifier of the session whose webhooks' calls are listed" optional:""`
	After   uint64    `json:"after,omitempty" help:"Serial number of the call after which they're listed, such as the last which was had" optional:""`
	Status  string    `json:"status,omitempty" help:"Which calls are listed: accepted, refused, or both, when it's empty" enum:"accepted,refused" optional:""`
	Since   time.Time `json:"since,omitzero" help:"When the calls which are listed were received, or after, to the second" example:"2026-11-24T09:00:00Z" optional:""`
	Newest  bool      `json:"newest,omitempty" help:"Whether the newest are listed first, rather than the oldest" optional:""`
	Limit   *uint64   `json:"limit,omitempty" help:"Most calls to return" optional:""`
}

// CallList is the calls which are listed, oldest first, or newest, and how
// many there are after the call, and since the time
type CallList struct {
	CallListRequest
	Count uint   `json:"count" help:"How many calls there are after the call, and since the time"`
	Body  []Call `json:"body,omitzero" help:"The calls"`
}

// TestRequest is the webhook which a test call is made to, and how it's
// signed
type TestRequest struct {
	Webhook   uint64 `json:"webhook" help:"Serial number of the webhook"`
	Signature string `json:"signature,omitempty" enum:"good,bad,none" help:"How the call is signed: good, with the webhook's secret, which it needs to have, bad, with another, or none. When it's empty, it's good, when the webhook has a secret, or else none" optional:""`
}

// HookResponse is the response to a call of a webhook's URL, which was kept,
// or was a delivery which was sent again
type HookResponse struct {
	Call      uint64 `json:"call,omitempty" help:"Serial number of the call, which was kept"`
	Duplicate bool   `json:"duplicate,omitempty" help:"Whether it was a delivery which was sent again, which wasn't kept again"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Query returns the request as query parameters
func (r CallListRequest) Query() url.Values {
	values := url.Values{}
	if r.Webhook > 0 {
		values.Set("webhook", strconv.FormatUint(r.Webhook, 10))
	}
	if session := strings.TrimSpace(r.Session); session != "" {
		values.Set("session", session)
	}
	if status := strings.TrimSpace(r.Status); status != "" {
		values.Set("status", status)
	}
	if r.After > 0 {
		values.Set("after", strconv.FormatUint(r.After, 10))
	}
	if !r.Since.IsZero() {
		values.Set("since", r.Since.UTC().Format(time.RFC3339Nano))
	}
	if r.Newest {
		values.Set("newest", "true")
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(*r.Limit, 10))
	}
	return values
}

// Status returns whether the call was accepted, or refused
func (c Call) Status() string {
	if c.Error != "" {
		return CallRefused
	}
	return CallAccepted
}

func (c Call) String() string {
	return types.Stringify(c)
}

func (l CallList) String() string {
	return types.Stringify(l)
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (Call) Header() []string {
	return []string{"ID", "Webhook", "Received", "Method", "Event", "Status"}
}

func (Call) Width(int) int {
	return 0
}

// Cell returns the call's serial number, its webhook's, when it was
// received, in local time, its method, what happened, and whether it was
// accepted, or refused, with why
func (c Call) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return c.ID
	case 1:
		return c.Webhook
	case 2:
		return c.Received.Local().Format(time.DateTime)
	case 3:
		return c.Method
	case 4:
		if c.Event == "" {
			return "-"
		}
		return c.Event
	case 5:
		status := CallAccepted
		if c.Error != "" {
			status = CallRefused + ": " + c.Error
		} else if c.Verified {
			status += ", verified"
		}
		if c.Test {
			status = "test " + status
		}
		return status
	}
	return ""
}
