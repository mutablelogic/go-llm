// Package heartbeat has prompts for sessions, which fire once, at a time, or
// again and again, on a schedule, such as a briefing every morning, or a
// reminder. A heartbeat which fires is run, with the function WithRun sets,
// once, on the server which fires it, with its prompt, and the session it's
// for. A schedule is in a time zone, or the person's, which it follows, as
// it changes.
package heartbeat

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"

	// The time zones, so they're known where the system has none, such as
	// in a container
	_ "time/tzdata"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Heartbeats are the heartbeats, which are kept in a store, and which fire,
// as Run runs. It's safe to use concurrently, and by more than one server,
// which share a store, so a heartbeat fires on only one of them.
type Heartbeats struct {
	store       Store
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	zone        func() *time.Location
	session     SessionFunc
	retention   time.Duration
	run         RunFunc
	now         func() time.Time

	// When the heartbeats which won't fire again were last looked for, which
	// only Run changes
	cleaned time.Time

	// The person's time zone, as it was, and when it changed, so a schedule
	// which follows it doesn't fire for a time before it changed, which, in
	// a zone which is ahead, has passed
	mu          sync.Mutex
	zoneSeen    bool
	zoneName    string
	zoneChanged time.Time
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Name is the schema of the heartbeats' events, and the namespace of
	// their tools
	Name = "heartbeat"

	// HeartbeatTable is the table of the events of the heartbeats, whose
	// rows are their identifiers
	HeartbeatTable = "heartbeat"

	// DefaultRetention is how long a heartbeat which won't fire again is
	// kept, after it last fired, or was changed, before it's deleted
	DefaultRetention = 7 * 24 * time.Hour
)

// errNotDue is returned by an update which fires a heartbeat, which isn't
// due, as it fired, such as on another server, or it changed
var errNotDue = errors.New("heartbeat isn't due")

// errNotExpired is returned by a delete which cleans up a heartbeat, which
// was changed since it was listed, so it isn't to be deleted yet
var errNotExpired = errors.New("heartbeat isn't expired")

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the heartbeats in the store, with the options, such as the
// broadcaster they publish their changes to, and the person's time zone.
// Returns ErrBadParameter when there's no store, or an option isn't valid.
func New(store Store, opts ...Opt) (*Heartbeats, error) {
	var o opt
	if store == nil {
		return nil, llm.ErrBadParameter.With("missing heartbeat store")
	} else if err := o.apply(opts); err != nil {
		return nil, err
	}
	if o.retention == 0 {
		o.retention = DefaultRetention
	}
	return &Heartbeats{store: store, broadcaster: o.broadcaster, tracer: o.tracer, zone: o.zone, session: o.session, retention: o.retention, run: o.run, now: time.Now}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create creates the heartbeat, publishes that it was, and returns it, with
// when it fires next. Returns ErrBadParameter when it isn't valid, or its
// time has passed, and the store's errors.
func (h *Heartbeats) Create(ctx context.Context, meta schema.HeartbeatMeta) (_ schema.Heartbeat, err error) {
	ctx, endSpan := otel.StartSpan(h.tracer, ctx, "heartbeat.Create",
		attribute.String("arg.meta", types.Stringify(meta)),
	)
	defer func() { endSpan(err) }()

	now := h.now()
	if meta, err = h.validate(meta, now, true); err != nil {
		return schema.Heartbeat{}, err
	} else if err := h.sessionExists(ctx, meta.Session); err != nil {
		return schema.Heartbeat{}, err
	}
	heartbeat := schema.Heartbeat{ID: uuid.New(), HeartbeatMeta: meta, Created: now.UTC(), Modified: now.UTC()}
	if err := h.store.Create(ctx, heartbeat); err != nil {
		return schema.Heartbeat{}, err
	}
	h.publish(eventschema.EventCreate, heartbeat)
	return h.withNext(heartbeat), nil
}

// Get returns the heartbeat with the identifier, with when it fires next, or
// ErrNotFound when there's none
func (h *Heartbeats) Get(ctx context.Context, id uuid.UUID) (_ schema.Heartbeat, err error) {
	ctx, endSpan := otel.StartSpan(h.tracer, ctx, "heartbeat.Get",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()

	heartbeat, err := h.store.Get(ctx, id)
	if err != nil {
		return schema.Heartbeat{}, err
	}
	return h.withNext(heartbeat), nil
}

// List returns the heartbeats, or those for the request's session, with when
// they fire next, the soonest first, and those which won't fire again last,
// of the request's page, and how many there are. Returns ErrBadParameter
// when the session isn't a UUID.
func (h *Heartbeats) List(ctx context.Context, req schema.HeartbeatListRequest) (_ schema.HeartbeatList, err error) {
	ctx, endSpan := otel.StartSpan(h.tracer, ctx, "heartbeat.List",
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	var session uuid.UUID
	if s := strings.TrimSpace(req.Session); s != "" {
		if session, err = uuid.Parse(s); err != nil {
			return schema.HeartbeatList{}, llm.ErrBadParameter.Withf("invalid session %q", s)
		}
	}
	heartbeats, err := h.store.List(ctx)
	if err != nil {
		return schema.HeartbeatList{}, err
	}
	result := make([]schema.Heartbeat, 0, len(heartbeats))
	for _, heartbeat := range heartbeats {
		if session == uuid.Nil() || heartbeat.Session == session {
			result = append(result, h.withNext(heartbeat))
		}
	}
	Sort(result)
	return req.Page(result), nil
}

// Update changes the heartbeat with the identifier, as the request says,
// publishes that it was, and returns it, with when it fires next. A
// heartbeat whose time is changed fires again. Returns ErrBadParameter when
// it isn't valid, or its new time has passed, ErrNotFound when there's none,
// and the store's errors.
func (h *Heartbeats) Update(ctx context.Context, id uuid.UUID, req schema.HeartbeatRequest) (_ schema.Heartbeat, err error) {
	ctx, endSpan := otel.StartSpan(h.tracer, ctx, "heartbeat.Update",
		attribute.String("arg.id", id.String()),
		attribute.String("arg.req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	now := h.now()
	heartbeat, err := h.store.Update(ctx, id, func(heartbeat *schema.Heartbeat) error {
		meta, err := h.validate(req.Apply(heartbeat.HeartbeatMeta), now, req.At != nil)
		if err != nil {
			return err
		}
		if meta.At != nil && (heartbeat.At == nil || !meta.At.Equal(*heartbeat.At)) {
			heartbeat.Fired = nil
		}
		heartbeat.HeartbeatMeta, heartbeat.Modified = meta, now.UTC()
		return nil
	})
	if err != nil {
		return schema.Heartbeat{}, err
	}
	h.publish(eventschema.EventUpdate, heartbeat)
	return h.withNext(heartbeat), nil
}

// Delete deletes the heartbeat with the identifier, publishes that it was,
// and returns it, as it was, or ErrNotFound when there's none
func (h *Heartbeats) Delete(ctx context.Context, id uuid.UUID) (_ schema.Heartbeat, err error) {
	ctx, endSpan := otel.StartSpan(h.tracer, ctx, "heartbeat.Delete",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()

	heartbeat, err := h.store.Delete(ctx, id, nil)
	if err != nil {
		return schema.Heartbeat{}, err
	}
	h.publish(eventschema.EventDelete, heartbeat)
	return heartbeat, nil
}

// DeleteSession deletes the heartbeats for the session, such as when it's
// deleted, publishing that they were, and returns how many there were
func (h *Heartbeats) DeleteSession(ctx context.Context, session uuid.UUID) (_ int, err error) {
	ctx, endSpan := otel.StartSpan(h.tracer, ctx, "heartbeat.DeleteSession",
		attribute.String("arg.session", session.String()),
	)
	defer func() { endSpan(err) }()

	heartbeats, err := h.store.List(ctx)
	if err != nil {
		return 0, err
	}
	var n int
	var errs []error
	for _, heartbeat := range heartbeats {
		if heartbeat.Session != session {
			continue
		}
		if _, err := h.store.Delete(ctx, heartbeat.ID, func(heartbeat schema.Heartbeat) error {
			// It's still the session's, as it is in the store
			if heartbeat.Session != session {
				return llm.ErrNotFound
			}
			return nil
		}); errors.Is(err, llm.ErrNotFound) {
			continue
		} else if err != nil {
			errs = append(errs, err)
			continue
		}
		h.publish(eventschema.EventDelete, heartbeat)
		n++
	}
	return n, errors.Join(errs...)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Sort sorts the heartbeats, those which fire soonest first, and those which
// won't fire again last, and then by when they were created
func Sort(heartbeats []schema.Heartbeat) {
	slices.SortFunc(heartbeats, func(a, b schema.Heartbeat) int {
		switch {
		case a.Next != nil && b.Next != nil:
			if c := a.Next.Compare(*b.Next); c != 0 {
				return c
			}
		case a.Next != nil:
			return -1
		case b.Next != nil:
			return 1
		}
		if c := a.Created.Compare(b.Created); c != 0 {
			return c
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// validate returns the heartbeat, as Validate does, or ErrBadParameter when
// its time is checked, as it's set, and has passed, by more than a minute,
// which says what time it is, in the person's time zone, when it's known,
// so a model which gave it can give a later one
func (h *Heartbeats) validate(meta schema.HeartbeatMeta, now time.Time, checkAt bool) (schema.HeartbeatMeta, error) {
	meta, err := meta.Validate()
	if err != nil {
		return schema.HeartbeatMeta{}, err
	} else if checkAt && meta.At != nil && meta.At.Before(now.Add(-time.Minute)) {
		loc, where := time.UTC, "UTC"
		if h.zone != nil {
			if zone := h.zone(); zone != nil {
				loc, where = zone, zone.String()
			}
		}
		return schema.HeartbeatMeta{}, llm.ErrBadParameter.Withf("heartbeat's time, %s, has passed, as it's now %s, in %s, so give a later time, with its offset", meta.At.In(loc).Format(time.RFC3339), now.In(loc).Truncate(time.Second).Format(time.RFC3339), where)
	}
	return meta, nil
}

// sessionExists returns nil when the session exists, or isn't checked, or
// ErrBadParameter when it doesn't exist
func (h *Heartbeats) sessionExists(ctx context.Context, session uuid.UUID) error {
	if h.session == nil {
		return nil
	} else if err := h.session(ctx, session); errors.Is(err, llm.ErrNotFound) {
		return llm.ErrBadParameter.Withf("there's no session %q", session)
	} else {
		return err
	}
}

// withNext returns the heartbeat, with when it fires next
func (h *Heartbeats) withNext(heartbeat schema.Heartbeat) schema.Heartbeat {
	heartbeat.Next = h.next(heartbeat)
	return heartbeat
}

// next returns when the heartbeat fires next, or nil, when it won't: its
// time, until it fires, or the next time on its schedule, in its time zone,
// or the person's, after it last fired, or was changed, or, when it follows
// the person's time zone, after it last changed
func (h *Heartbeats) next(heartbeat schema.Heartbeat) *time.Time {
	if heartbeat.At != nil {
		if heartbeat.Fired != nil {
			return nil
		}
		return types.Ptr(heartbeat.At.UTC())
	}
	spec, err := schema.ParseTimeSpec(heartbeat.Schedule)
	if err != nil {
		return nil
	}

	// After it was changed, or last fired, in the minute after, so it
	// doesn't fire again in the same minute
	from := heartbeat.Modified
	if heartbeat.Fired != nil {
		if after := heartbeat.Fired.Truncate(time.Minute).Add(time.Minute); after.After(from) {
			from = after
		}
	}
	loc := time.UTC
	if heartbeat.Timezone != "" {
		if zone, err := schema.LoadLocation(heartbeat.Timezone); err == nil {
			loc = zone
		}
	} else {
		zone, changed := h.personZone()
		if zone != nil {
			loc = zone
		}
		if changed.After(from) {
			from = changed
		}
	}
	if next := spec.Next(from, loc); !next.IsZero() {
		return types.Ptr(next.UTC())
	}
	return nil
}

// personZone returns the person's time zone, or nil, when it isn't known,
// and when it changed, which is when it was first seen to be different
func (h *Heartbeats) personZone() (*time.Location, time.Time) {
	if h.zone == nil {
		return nil, time.Time{}
	}
	zone := h.zone()
	name := ""
	if zone != nil {
		name = zone.String()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.zoneSeen {
		h.zoneSeen, h.zoneName = true, name
	} else if name != h.zoneName {
		h.zoneName, h.zoneChanged = name, h.now()
	}
	return zone, h.zoneChanged
}

// publish publishes the change to the heartbeat, when there's a broadcaster
func (h *Heartbeats) publish(action eventschema.EventAction, heartbeat schema.Heartbeat) {
	if h.broadcaster == nil {
		return
	}
	h.broadcaster.Publish(eventschema.Event{Action: action, Schema: Name, Table: HeartbeatTable, Row: heartbeat.ID.String()})
}
