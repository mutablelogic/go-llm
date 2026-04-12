package heartbeat

import (
	"context"
	"strings"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	hschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	kernel "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CreateRequest describes the manager-level input for creating a heartbeat.
type CreateRequest struct {
	Session  uuid.UUID `json:"session"`
	Message  string    `json:"message"`
	Schedule string    `json:"schedule"`
	Timezone string    `json:"timezone,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create validates a request, converts the schedule into a TimeSpec, and inserts a new heartbeat.
func (m *Manager) Create(ctx context.Context, req CreateRequest) (_ *hschema.Heartbeat, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateHeartbeat",
		attribute.String("req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	insert, err := req.HeartbeatInsert()
	if err != nil {
		return nil, err
	}

	var result hschema.Heartbeat
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		return conn.Insert(ctx, &result, insert)
	}); err != nil {
		return nil, pg.NormalizeError(err)
	}

	return types.Ptr(result), nil
}

// HeartbeatInsert validates the create request and converts it into a schema insert payload.
func (req CreateRequest) HeartbeatInsert() (_ hschema.HeartbeatInsert, err error) {
	if req.Session == uuid.Nil {
		return hschema.HeartbeatInsert{}, kernel.ErrBadParameter.With("session is required")
	}

	loc, err := loadHeartbeatLocation(req.Timezone)
	if err != nil {
		return hschema.HeartbeatInsert{}, err
	}

	schedule, err := hschema.NewTimeSpec(strings.TrimSpace(req.Schedule), loc)
	if err != nil {
		return hschema.HeartbeatInsert{}, err
	}

	return hschema.HeartbeatInsert{
		Session: req.Session,
		HeartbeatMeta: hschema.HeartbeatMeta{
			Message:  req.Message,
			Schedule: schedule,
		},
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func loadHeartbeatLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	if name == "Local" {
		return nil, kernel.ErrBadParameter.With("timezone must be a specific IANA name (e.g. Europe/London), not \"Local\"")
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, kernel.ErrBadParameter.Withf("unknown timezone %q: %v", name, err)
	}
	return loc, nil
}
