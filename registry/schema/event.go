package schema

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// EventStatus is what's become of an event's message: it's ignored, as
// there's nothing worth telling the person, pending, until it's delivered,
// notified, as the person was told, such as by an alert, or delivered, into
// the conversation it's for
type EventStatus string

// EventImportance is how important an event's message is: whether it's
// ignored, the person is notified, or alerted
type EventImportance string

// EventMeta is the outcome of an event, such as a heartbeat which fired,
// which was run: the session it ran in, the conversation it's for, what it's
// for, when it was due, and fired, and the message, how important it is,
// and what's become of it
type EventMeta struct {
	Run        uuid.UUID       `json:"run" help:"Identifier of the session the event ran in, which has what it did" example:"3b241101-e2bb-4255-8caf-4136c566a962"`
	Session    uuid.UUID       `json:"session" help:"Identifier of the session the event is for, which its message is delivered to" example:"9a6c0b4e-1d2f-4c3b-8e7a-5f6d4c3b2a10"`
	Title      string          `json:"title,omitempty" help:"What the event is for" example:"Morning briefing"`
	Message    string          `json:"message" help:"What the person is told, or, when it's ignored, why" example:"It's 4°C and cloudy in Berlin today"`
	Importance EventImportance `json:"importance" help:"How important the message is" enum:"ignore,notify,alert" example:"notify"`
	Status     EventStatus     `json:"status" help:"What's become of the message" enum:"ignored,pending,notified,delivered" example:"pending"`
	Scheduled  time.Time       `json:"scheduled,omitzero" help:"When the event was due"`
	Fired      time.Time       `json:"fired,omitzero" help:"When the event fired, which is later than it was due, when it's late"`
}

// Event is the outcome of an event, as it's stored: its identifier, and
// serial, which its store changes each time it's stored, so a change is made
// to it as it was last seen, and when it was created, and last changed
type Event struct {
	ID     uuid.UUID `json:"id" readonly:"" help:"Identifier of the event" example:"0f8fad5b-d9cb-469f-a165-70867728950e"`
	Serial uint64    `json:"serial,omitempty" readonly:"" help:"Serial of the event, which its store changes each time it's stored" example:"1"`
	EventMeta
	Created  time.Time `json:"created,omitzero" readonly:"" help:"When the event's outcome was stored"`
	Modified time.Time `json:"modified,omitzero" readonly:"" help:"When the event's outcome was last changed"`
}

// EventListRequest selects events: those for a session, and of a status,
// when they're given, from an offset, and at most a limit of them
type EventListRequest struct {
	Session string      `json:"session,omitempty" help:"Identifier of the session the events are for, or every session" optional:""`
	Status  EventStatus `json:"status,omitempty" help:"Status of the events, or every status" enum:"ignored,pending,notified,delivered" optional:""`
	Offset  uint        `json:"offset,omitempty" help:"Events to skip" example:"0" optional:""`
	Limit   *uint       `json:"limit,omitempty" help:"Most events to return" example:"10" optional:""`
}

// EventList is the events a request selects, the most recent first, and how
// many there are
type EventList struct {
	EventListRequest
	Count uint    `json:"count" help:"How many events there are, of the session, and status" example:"42"`
	Body  []Event `json:"body,omitempty" help:"The events, the most recent first"`
}

// EventChange is the data of the registry's event of an event's outcome,
// which was created, or changed: its serial, and status
type EventChange struct {
	Serial uint64      `json:"serial,omitempty" help:"Serial of the event, as it is now"`
	Status EventStatus `json:"status,omitempty" help:"Status of the event, as it is now"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	EventIgnored   EventStatus = "ignored"   // There's nothing worth telling the person
	EventPending   EventStatus = "pending"   // It's to be delivered
	EventNotified  EventStatus = "notified"  // The person was told, such as by an alert
	EventDelivered EventStatus = "delivered" // It's in the conversation it's for
)

const (
	ImportanceIgnore EventImportance = "ignore" // There's nothing worth telling the person
	ImportanceNotify EventImportance = "notify" // It's worth telling them
	ImportanceAlert  EventImportance = "alert"  // Acting late would cost them something
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ParseEventStatus returns the status, or ErrBadParameter when it isn't one
func ParseEventStatus(s string) (EventStatus, error) {
	switch status := EventStatus(strings.TrimSpace(s)); status {
	case EventIgnored, EventPending, EventNotified, EventDelivered:
		return status, nil
	}
	return "", llm.ErrBadParameter.Withf("invalid event status %q, which isn't ignored, pending, notified or delivered", s)
}

// ParseEventImportance returns the importance, which is notify when it's
// empty, or ErrBadParameter when it isn't one
func ParseEventImportance(s string) (EventImportance, error) {
	switch importance := EventImportance(strings.TrimSpace(s)); importance {
	case "":
		return ImportanceNotify, nil
	case ImportanceIgnore, ImportanceNotify, ImportanceAlert:
		return importance, nil
	}
	return "", llm.ErrBadParameter.Withf("invalid importance %q, which isn't ignore, notify or alert", s)
}

// Status returns the status of an event of the importance, when it's run,
// which is ignored, when it's ignored, and otherwise pending, until it's
// delivered
func (i EventImportance) Status() EventStatus {
	if i == ImportanceIgnore {
		return EventIgnored
	}
	return EventPending
}

// SortEvents sorts the events, the most recently created first, and then by
// their identifiers, as a store lists them
func SortEvents(events []Event) {
	slices.SortFunc(events, func(a, b Event) int {
		if c := b.Created.Compare(a.Created); c != 0 {
			return c
		}
		return a.ID.Compare(b.ID)
	})
}

// Matches returns whether the event is of the request's session, and
// status, when it has them
func (r EventListRequest) Matches(event Event) bool {
	if session := strings.TrimSpace(r.Session); session != "" && event.Session.String() != session {
		return false
	}
	return r.Status == "" || event.Status == r.Status
}

// Query returns the request as the parameters of a URL's query
func (r EventListRequest) Query() url.Values {
	values := url.Values{}
	if session := strings.TrimSpace(r.Session); session != "" {
		values.Set("session", session)
	}
	if r.Status != "" {
		values.Set("status", string(r.Status))
	}
	if r.Offset > 0 {
		values.Set("offset", strconv.FormatUint(uint64(r.Offset), 10))
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(uint64(*r.Limit), 10))
	}
	return values
}

func (e Event) String() string {
	return types.Stringify(e)
}

func (r EventListRequest) String() string {
	return types.Stringify(r)
}

func (l EventList) String() string {
	return types.Stringify(l)
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (Event) Header() []string {
	return []string{"ID", "Title", "Importance", "Status", "Fired"}
}

func (Event) Width(int) int {
	return 0
}

// Cell returns the event's identifier, title, importance, status, and when
// it fired, in local time
func (e Event) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return e.ID.String()
	case 1:
		return e.Title
	case 2:
		return string(e.Importance)
	case 3:
		return string(e.Status)
	case 4:
		if e.Fired.IsZero() {
			return ""
		}
		return e.Fired.Local().Format(time.DateTime)
	}
	return ""
}
