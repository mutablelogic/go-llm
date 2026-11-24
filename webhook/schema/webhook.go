// Package schema has the webhooks: what each is, by its serial number, the
// session its calls are for, and its description
package schema

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"uuid"

	// Packages
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// WebhookMeta is what a webhook is for, which is set when it's registered:
// the session its calls are for, its description, the protocol of the
// service which calls it, and the secret which the service signs its calls
// with, when it does
type WebhookMeta struct {
	Session     uuid.UUID `json:"session" help:"Session which the webhook's calls are for"`
	Description string    `json:"description" help:"What the webhook is for, such as which service calls it, and when, which it needs" example:"GitHub: issues opened on go-llm"`
	Type        string    `json:"type,omitempty" help:"Protocol of the service which calls it, such as github, which says how its calls are verified, and what they are, or generic, when it's empty" example:"github" optional:""`
	Secret      string    `json:"secret,omitempty" writeonly:"" help:"Secret which the service signs its calls with, which they're verified with, when it's set, which isn't returned" optional:""`
}

// Webhook is a webhook, its serial number, when it was registered, and
// whether it has a secret, which isn't returned
type Webhook struct {
	ID uint64 `json:"id" readonly:"" help:"Serial number of the webhook"`
	WebhookMeta
	HasSecret bool      `json:"has_secret" readonly:"" help:"Whether it has a secret, which its calls are verified with"`
	Created   time.Time `json:"created,omitzero" readonly:"" help:"When it was registered"`
}

// WebhookToken is a webhook which was registered, and its token, which is in
// the URL its service calls, which is only returned now, as it isn't kept
type WebhookToken struct {
	Webhook
	Token string `json:"token" readonly:"" help:"Token of the URL its service calls, which isn't returned again"`
	URL   string `json:"url,omitempty" readonly:"" help:"URL its service calls, of the server's public endpoint, which isn't returned again" example:"https://llm.example.com/api/hook/7hW2…"`
}

// WebhookRequest is a change to a webhook: what's set is changed, such as
// its secret, which the service gives, once it's registered there, and an
// empty secret removes it
type WebhookRequest struct {
	Description *string `json:"description,omitempty" help:"What the webhook is for" optional:""`
	Secret      *string `json:"secret,omitempty" writeonly:"" help:"Secret which the service signs its calls with, or empty, to remove it" optional:""`
}

// WebhookListRequest is which webhooks are listed: those of the session,
// when there is one, from the offset, and at most the limit of them
type WebhookListRequest struct {
	Session string  `json:"session,omitempty" help:"Identifier of the session whose webhooks are listed, or every webhook" optional:""`
	Offset  uint64  `json:"offset,omitempty" help:"How many webhooks to skip" optional:""`
	Limit   *uint64 `json:"limit,omitempty" help:"Most webhooks to return" optional:""`
}

// WebhookList is the webhooks which are listed, newest first, and how many
// there are
type WebhookList struct {
	WebhookListRequest
	Count uint      `json:"count" help:"How many webhooks there are"`
	Body  []Webhook `json:"body,omitzero" help:"The webhooks"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Query returns the request as query parameters
func (r WebhookListRequest) Query() url.Values {
	values := url.Values{}
	if session := strings.TrimSpace(r.Session); session != "" {
		values.Set("session", session)
	}
	if r.Offset > 0 {
		values.Set("offset", strconv.FormatUint(r.Offset, 10))
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(*r.Limit, 10))
	}
	return values
}

func (w Webhook) String() string {
	return types.Stringify(w)
}

func (w WebhookToken) String() string {
	return types.Stringify(w)
}

func (l WebhookList) String() string {
	return types.Stringify(l)
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (Webhook) Header() []string {
	return []string{"ID", "Type", "Description", "Session", "Secret", "Created"}
}

func (Webhook) Width(int) int {
	return 0
}

// Cell returns the webhook's serial number, type, description, session,
// whether it has a secret, and when it was created, in local time
func (w Webhook) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return w.ID
	case 1:
		return w.Type
	case 2:
		return w.Description
	case 3:
		return w.Session.String()
	case 4:
		if w.HasSecret {
			return "yes"
		}
		return "no"
	case 5:
		return w.Created.Local().Format(time.DateTime)
	}
	return ""
}
