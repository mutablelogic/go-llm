package schema

import (
	"strings"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llmschema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// HeartbeatIDSelector selects a single heartbeat by ID for get/update/delete operations.
type HeartbeatIDSelector string

type HeartbeatMeta struct {
	Message  string   `json:"message"`
	Schedule TimeSpec `json:"schedule"`
}

type Heartbeat struct {
	ID string `json:"id"`
	HeartbeatMeta
	Fired     bool       `json:"fired,omitempty"`
	LastFired *time.Time `json:"last_fired,omitempty"`
	Created   time.Time  `json:"created"`
	Modified  *time.Time `json:"modified,omitempty"`
}

// HeartbeatInsert contains the fields required to create a new heartbeat row.
type HeartbeatInsert struct {
	Session uuid.UUID `json:"session"`
	HeartbeatMeta
}

// HeartbeatListRequest is the request type for listing heartbeats.
type HeartbeatListRequest struct {
	pg.OffsetLimit
	Fired *bool `json:"fired,omitempty"`
}

// HeartbeatList is a pg.Reader that accumulates scanned heartbeat rows.
type HeartbeatList struct {
	HeartbeatListRequest
	Count uint         `json:"count"`
	Body  []*Heartbeat `json:"body,omitempty"`
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

// HeartbeatMarkFiredSelector selects a heartbeat by ID for the mark-fired update.
type HeartbeatMarkFiredSelector string

func (id HeartbeatMarkFiredSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	bind.Set("id", string(id))
	return bind.Query("heartbeat.mark_fired"), nil
}

func (h HeartbeatListRequest) Select(bind *pg.Bind, op pg.Op) (string, error) {
	// Set offset and limit for pagination
	h.OffsetLimit.Bind(bind, HeartbeatListRequestMax)

	// Set WHERE phrases
	bind.Del("where")
	if h.Fired != nil {
		bind.Append("where", "fired="+bind.Set("fired", types.Value(h.Fired)))
	}

	where := bind.Join("where", " AND ")

	// Return the SQL query name for this operation
	switch op {
	case pg.List:
		if user := bind.Get("user"); user != nil && user.(uuid.UUID) != uuid.Nil {
			if where != "" {
				bind.Set("where", `AND `+where)
			} else {
				bind.Set("where", "")
			}
			return bind.Query("heartbeat.list_for_user"), nil
		} else {
			if where != "" {
				bind.Set("where", `WHERE `+where)
			} else {
				bind.Set("where", "")
			}
			return bind.Query("heartbeat.list"), nil
		}
	default:
		return "", llmschema.ErrInternalServerError.Withf("Unsupported HeartbeatListRequest operation %q", op)
	}
}

///////////////////////////////////////////////////////////////////////////////
// READERS

func (h *Heartbeat) Scan(row pg.Row) error {
	return row.Scan(&h.ID, &h.Message, &h.Schedule, &h.Fired, &h.LastFired, &h.Created, &h.Modified)
}

func (h *HeartbeatList) Scan(row pg.Row) error {
	var hb Heartbeat
	if err := hb.Scan(row); err != nil {
		return err
	}
	h.Body = append(h.Body, &hb)
	return nil
}

func (h *HeartbeatList) ScanCount(row pg.Row) error {
	return row.Scan(&h.Count)
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
	if session, _ := bind.Get("session").(uuid.UUID); session == uuid.Nil {
		return "", llmschema.ErrBadParameter.With("session is required")
	}

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

func (h HeartbeatInsert) Insert(bind *pg.Bind) (string, error) {
	if h.Session == uuid.Nil {
		return "", llmschema.ErrBadParameter.With("session is required")
	}
	bind.Set("session", h.Session)
	return h.HeartbeatMeta.Insert(bind)
}
