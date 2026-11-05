// Package schema has the heartbeats' schedules, and what they are: a prompt
// for a session, at a time, or on a schedule
package schema

import (
	"net/url"
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

// HeartbeatMeta is a prompt for a session, which fires once, at a time, or on
// a schedule, in a time zone, or the person's, when it has none, so it
// follows them, as their time zone changes
type HeartbeatMeta struct {
	Title    string     `json:"title" help:"What it's for, in a few words" example:"Morning briefing"`
	Prompt   string     `json:"prompt" help:"The instruction for the model, when it fires, which is all it's told, so it says everything that's needed" example:"Give David the weather in Berlin today, and the three top news headlines, briefly"`
	Session  uuid.UUID  `json:"session" help:"Session which the prompt is for"`
	Schedule string     `json:"schedule,omitempty" help:"When it fires, again and again, as a cron expression, minute hour day month weekday, such as 0 9 * * 1-5 for nine on weekdays" example:"0 8 * * 1-5" optional:""`
	At       *time.Time `json:"at,omitempty" help:"When it fires, once, with its offset, such as 2026-11-06T15:00:00+01:00" optional:""`
	Timezone string     `json:"timezone,omitempty" help:"Time zone of the schedule, by its IANA name, such as Europe/Berlin, or none, for the person's, as it changes" example:"Europe/Berlin" optional:""`
}

// Heartbeat is a heartbeat, its identifier, when it was created and last
// changed, when it last fired, and when it fires next, when it does
type Heartbeat struct {
	ID uuid.UUID `json:"id" readonly:"" help:"Identifier of the heartbeat"`
	HeartbeatMeta
	Created  time.Time  `json:"created,omitzero" readonly:"" help:"When it was created"`
	Modified time.Time  `json:"modified,omitzero" readonly:"" help:"When it was last changed"`
	Fired    *time.Time `json:"fired,omitempty" readonly:"" help:"When it last fired"`
	Next     *time.Time `json:"next,omitempty" readonly:"" help:"When it fires next, or none, when it won't, as it fired once"`
}

// HeartbeatRequest is a change to a heartbeat: what's set is changed, and a
// schedule replaces a time, and a time a schedule, and an empty time zone
// follows the person's
type HeartbeatRequest struct {
	Title    *string    `json:"title,omitempty" help:"What it's for, in a few words" optional:""`
	Prompt   *string    `json:"prompt,omitempty" help:"The instruction for the model, when it fires" optional:""`
	Schedule *string    `json:"schedule,omitempty" help:"When it fires, again and again, as a cron expression, which replaces its time" optional:""`
	At       *time.Time `json:"at,omitempty" help:"When it fires, once, which replaces its schedule" optional:""`
	Timezone *string    `json:"timezone,omitempty" help:"Time zone of its schedule, or empty, for the person's" optional:""`
}

// HeartbeatListRequest is which heartbeats are listed: those for the
// session, when there is one, from the offset, and at most the limit of them
type HeartbeatListRequest struct {
	Session string  `json:"session,omitempty" help:"Identifier of the session whose heartbeats are listed, or every heartbeat" optional:""`
	Offset  uint64  `json:"offset,omitempty" help:"How many heartbeats to skip" optional:""`
	Limit   *uint64 `json:"limit,omitempty" help:"Most heartbeats to return" optional:""`
}

// HeartbeatList is the heartbeats which are listed, which fire soonest
// first, and those which won't fire again last, and how many there are
type HeartbeatList struct {
	HeartbeatListRequest
	Count uint        `json:"count" help:"How many heartbeats there are"`
	Body  []Heartbeat `json:"body,omitzero" help:"The heartbeats"`
}

// FireEvent is the data of the event of a heartbeat which fires: what it is,
// when it was due, and when it fired, which is later, such as when the
// server wasn't running, so what acts on it can say it's late, or not act
type FireEvent struct {
	Heartbeat uuid.UUID `json:"heartbeat" help:"Identifier of the heartbeat"`
	Title     string    `json:"title" help:"What it's for"`
	Prompt    string    `json:"prompt" help:"The instruction for the model"`
	Session   uuid.UUID `json:"session" help:"Session which the prompt is for"`
	Scheduled time.Time `json:"scheduled" help:"When it was due"`
	Fired     time.Time `json:"fired" help:"When it fired"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// HeartbeatListMax is the most heartbeats which are listed at once
const HeartbeatListMax uint64 = 100

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - HEARTBEAT

// Validate returns the heartbeat, with its title and prompt trimmed, and its
// time in UTC, or ErrBadParameter when it has no title, or a title of more
// than one line, no prompt, or session, neither a schedule nor a time, or
// both, or a time zone which isn't known, or a time zone with a time, which
// has its offset
func (m HeartbeatMeta) Validate() (HeartbeatMeta, error) {
	m.Title = strings.TrimSpace(m.Title)
	m.Prompt = strings.TrimSpace(m.Prompt)
	m.Timezone = strings.TrimSpace(m.Timezone)
	m.Schedule = strings.TrimSpace(m.Schedule)
	switch {
	case m.Title == "":
		return HeartbeatMeta{}, llm.ErrBadParameter.With("heartbeat has no title")
	case strings.ContainsAny(m.Title, "\r\n"):
		return HeartbeatMeta{}, llm.ErrBadParameter.With("heartbeat has a title of more than one line")
	case m.Prompt == "":
		return HeartbeatMeta{}, llm.ErrBadParameter.With("heartbeat has no prompt")
	case m.Session == uuid.Nil():
		return HeartbeatMeta{}, llm.ErrBadParameter.With("heartbeat has no session")
	case m.Schedule == "" && m.At == nil:
		return HeartbeatMeta{}, llm.ErrBadParameter.With("heartbeat has neither a schedule, nor a time")
	case m.Schedule != "" && m.At != nil:
		return HeartbeatMeta{}, llm.ErrBadParameter.With("heartbeat has a schedule, and a time, rather than one of them")
	case m.At != nil && m.Timezone != "":
		return HeartbeatMeta{}, llm.ErrBadParameter.With("heartbeat has a time zone, and a time, which has its offset")
	}
	if m.Schedule != "" {
		schedule, err := ParseTimeSpec(m.Schedule)
		if err != nil {
			return HeartbeatMeta{}, err
		}
		m.Schedule = schedule.String()
	}
	if m.Timezone != "" {
		if _, err := LoadLocation(m.Timezone); err != nil {
			return HeartbeatMeta{}, err
		}
	}
	if m.At != nil {
		m.At = types.Ptr(m.At.UTC())
	}
	return m, nil
}

// Apply returns the heartbeat, with the changes of the request
func (r HeartbeatRequest) Apply(m HeartbeatMeta) HeartbeatMeta {
	if r.Title != nil {
		m.Title = *r.Title
	}
	if r.Prompt != nil {
		m.Prompt = *r.Prompt
	}
	if r.Schedule != nil {
		m.Schedule, m.At = *r.Schedule, nil
	}
	if r.At != nil {
		m.At, m.Schedule, m.Timezone = r.At, "", ""
	}
	if r.Timezone != nil {
		m.Timezone = *r.Timezone
	}
	return m
}

// When returns when the heartbeat fires: its schedule, and its time zone,
// when it has one, or its time
func (m HeartbeatMeta) When() string {
	switch {
	case m.At != nil:
		return m.At.Format(time.RFC3339)
	case m.Schedule != "" && m.Timezone != "":
		return m.Schedule + " " + m.Timezone
	default:
		return m.Schedule
	}
}

func (h Heartbeat) String() string {
	return types.Stringify(h)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - LIST

// Query returns the request as URL query values
func (r HeartbeatListRequest) Query() url.Values {
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

// Page returns the heartbeats of the request's page, from its offset, and
// at most its limit, or HeartbeatListMax, of them, and how many there are
func (r HeartbeatListRequest) Page(heartbeats []Heartbeat) HeartbeatList {
	limit := HeartbeatListMax
	if r.Limit != nil {
		limit = min(*r.Limit, HeartbeatListMax)
	}
	list := HeartbeatList{HeartbeatListRequest: r, Count: uint(len(heartbeats))}
	if r.Offset < uint64(len(heartbeats)) {
		heartbeats = heartbeats[r.Offset:]
		list.Body = heartbeats[:min(uint64(len(heartbeats)), limit)]
	}
	return list
}

func (r HeartbeatListRequest) String() string {
	return types.Stringify(r)
}

func (l HeartbeatList) String() string {
	return types.Stringify(l)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// LoadLocation returns the time zone with the IANA name, such as
// Europe/Berlin, or ErrBadParameter when it isn't known, or is Local, which
// is the server's, rather than the person's
func LoadLocation(name string) (*time.Location, error) {
	if name == "Local" {
		return nil, llm.ErrBadParameter.With("time zone must be an IANA name, such as Europe/Berlin, not Local")
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("unknown time zone %q, which must be an IANA name, such as Europe/Berlin", name)
	}
	return loc, nil
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (Heartbeat) Header() []string {
	return []string{"ID", "Title", "When", "Next"}
}

func (Heartbeat) Width(int) int {
	return 0
}

// Cell returns the heartbeat's identifier, title, when it fires, and when it
// fires next, in local time
func (h Heartbeat) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return h.ID.String()
	case 1:
		return h.Title
	case 2:
		return h.When()
	case 3:
		if h.Next == nil {
			return "-"
		}
		return h.Next.Local().Format(time.DateTime)
	}
	return ""
}
