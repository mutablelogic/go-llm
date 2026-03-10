package heartbeat

import "time"

type Heartbeat struct {
	ID        string     `json:"id"`
	Message   string     `json:"message"`
	Schedule  TimeSpec   `json:"schedule"`
	Fired     bool       `json:"fired,omitempty"`
	LastFired *time.Time `json:"last_fired,omitempty"`
	Created   time.Time  `json:"created"`
	Modified  time.Time  `json:"modified"`
}

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

// Store is the interface for a heartbeat store.
type Store interface {
	Create(message string, schedule TimeSpec) (*Heartbeat, error)
	Get(id string) (*Heartbeat, error)
	Delete(id string) error
	List(includeFired bool) ([]*Heartbeat, error)
	Update(id, message string, schedule *TimeSpec) (*Heartbeat, error)
	MarkFired(id string) error
	Due() ([]*Heartbeat, error)
}
