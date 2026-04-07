package schema

import (
	"strings"
	"time"

	// Packages
	llmschema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// STORAGE TYPES

// HeartbeatIDSelector selects a single heartbeat by ID for get/update/delete operations.
type HeartbeatIDSelector string

type HeartbeatMeta struct {
	Message  string         `json:"message"`
	Schedule TimeSpec       `json:"schedule"`
	Meta     map[string]any `json:"meta,omitempty"`
}

type Heartbeat struct {
	ID string `json:"id"`
	HeartbeatMeta
	Fired     bool       `json:"fired,omitempty"`
	LastFired *time.Time `json:"last_fired,omitempty"`
	Created   time.Time  `json:"created"`
	Modified  *time.Time `json:"modified,omitempty"`
}

// HeartbeatListRequest is the request type for listing heartbeats.
type HeartbeatListRequest struct {
	Fired *bool `json:"fired,omitempty"`
}

// HeartbeatList is a pg.Reader that accumulates scanned heartbeat rows.
type HeartbeatList struct {
	Heartbeats []*Heartbeat `json:"body,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// ARGUMENT TYPES

type AddHeartbeatRequest struct {
	Message  string `json:"message"            jsonschema:"Short reminder message to deliver when the heartbeat matures."`
	Schedule string `json:"schedule"           jsonschema:"When and how often to fire: RFC 3339 timestamp (once, e.g. 2026-06-01T15:00:00Z) or 5-field cron expression like '0 9 * * 1-5' (recurring)."`
	Timezone string `json:"timezone,omitempty" jsonschema:"Optional IANA timezone for evaluating the schedule (e.g. Europe/London, America/New_York). Not needed for RFC 3339 timestamps that already carry a timezone offset. Cron expressions default to UTC when omitted."`
}

type DeleteHeartbeatRequest struct {
	ID string `json:"id" jsonschema:"The unique ID of the heartbeat to delete."`
}

type ListHeartbeatsRequest struct {
	IncludeFired bool `json:"include_fired,omitempty" jsonschema:"Include already-fired heartbeats."`
}

type UpdateHeartbeatRequest struct {
	ID       string `json:"id"                 jsonschema:"The unique ID of the heartbeat to update."`
	Message  string `json:"message,omitempty"  jsonschema:"New message; empty keeps existing."`
	Schedule string `json:"schedule,omitempty" jsonschema:"New schedule (RFC 3339 timestamp or 5-field cron expression); empty keeps existing."`
	Timezone string `json:"timezone,omitempty" jsonschema:"New IANA timezone for evaluating the schedule (e.g. Europe/London). Not needed for RFC 3339 timestamps that already carry a timezone offset. Cron expressions default to UTC when omitted."`
}

///////////////////////////////////////////////////////////////////////////////
// SELECTORS

func (id HeartbeatIDSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	bind.Set("id", string(id))
	switch op {
	case pg.Get:
		return bind.Query("heartbeat.select"), nil
	case pg.Update:
		return bind.Query("heartbeat.update"), nil
	case pg.Delete:
		return bind.Query("heartbeat.delete"), nil
	default:
		return "", llmschema.ErrInternalServerError.Withf("unsupported HeartbeatIDSelector operation %q", op)
	}
}

// HeartbeatMarkFiredSelector selects a heartbeat by ID for the mark-fired UPDATE.
type HeartbeatMarkFiredSelector string

func (id HeartbeatMarkFiredSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	bind.Set("id", string(id))
	return bind.Query("heartbeat.mark_fired"), nil
}

func (h HeartbeatListRequest) Select(bind *pg.Bind, op pg.Op) (string, error) {
	// Set WHERE phrases
	bind.Del("where")
	if h.Fired != nil {
		bind.Append("where", "fired="+bind.Set("fired", types.Value(h.Fired)))
	}

	// Join WHERE clauses with AND
	if where := bind.Join("where", " AND "); where != "" {
		bind.Set("where", `WHERE `+where)
	} else {
		bind.Set("where", "")
	}

	// We limit ourselves to 100 results per request to prevent overload; clients can page through results by filtering on created/modified timestamps via the Meta field.
	bind.Set("offsetlimit", "LIMIT 100")

	// Return the SQL query name for this operation
	switch op {
	case pg.List:
		return bind.Query("heartbeat.list"), nil
	default:
		return "", llmschema.ErrInternalServerError.Withf("Unsupported HeartbeatListRequest operation %q", op)
	}
}

///////////////////////////////////////////////////////////////////////////////
// READERS

func (h *Heartbeat) Scan(row pg.Row) error {
	return row.Scan(&h.ID, &h.Message, &h.Schedule, &h.Fired, &h.LastFired, &h.Created, &h.Modified, &h.Meta)
}

func (h *HeartbeatList) Scan(row pg.Row) error {
	var hb Heartbeat
	if err := hb.Scan(row); err != nil {
		return err
	}
	h.Heartbeats = append(h.Heartbeats, &hb)
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// WRITERS

func (h HeartbeatMeta) Update(bind *pg.Bind) error {
	// Set patch fields for non-zero values
	bind.Del("patch")

	// Message
	if message := strings.TrimSpace(h.Message); message != "" {
		bind.Append("patch", `message = `+bind.Set("message", message))

	}

	// Timespec
	if !h.Schedule.IsZero() {
		bind.Append("patch", `schedule = `+bind.Set("schedule", h.Schedule))

		// Reset fired and last_fired only if the new schedule has a future occurrence.
		// If the schedule is already in the past (one-shot that's passed), mark it fired.
		if h.Schedule.Next(time.Now()).IsZero() {
			bind.Append("patch", `fired = TRUE`)
		} else {
			bind.Append("patch", `fired = FALSE`)
			bind.Append("patch", `last_fired = NULL`)
		}
	}

	// Error on updating meta for the moment
	if len(h.Meta) > 0 {
		return llmschema.ErrBadParameter.With("updating meta is not supported")
	}

	// Check that there's at least one field to update before adding modified timestamp
	if bind.Join("patch", ", ") == "" {
		return llmschema.ErrBadParameter.With("no fields to update")
	}

	// Always update modified timestamp
	bind.Append("patch", `modified = NOW()`)

	// Set patch
	bind.Set("patch", bind.Join("patch", ", "))

	// Return success
	return nil
}

func (h HeartbeatMeta) Insert(bind *pg.Bind) (string, error) {
	// Message
	if message := strings.TrimSpace(h.Message); message == "" {
		return "", llmschema.ErrBadParameter.With("message is required")
	} else {
		bind.Set("message", h.Message)
	}

	// Schedule
	bind.Set("schedule", h.Schedule)

	// Return the SQL query name for this operation
	return bind.Query("heartbeat.insert"), nil
}
