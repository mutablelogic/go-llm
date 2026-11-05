package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// tool is the tool which schedules, lists, changes and cancels heartbeats,
// as its operation says, for the session which calls it
type tool struct {
	toolkit.Base
	heartbeats *Heartbeats
}

var _ llm.Tool = (*tool)(nil)
var _ toolkit.Namespace = (*Heartbeats)(nil)

// toolInput is the input of the heartbeats tool: its operation, and what the
// operation needs
type toolInput struct {
	Operation string `json:"operation" enum:"schedule,list,update,cancel" help:"schedule a heartbeat, list the conversation's heartbeats, update a heartbeat, by its id, or cancel it"`
	ID        string `json:"id,omitempty" help:"Identifier of the heartbeat, to update or cancel" optional:""`
	Title     string `json:"title,omitempty" help:"What it's for, in a few words, such as Morning briefing, to schedule, or update" optional:""`
	Prompt    string `json:"prompt,omitempty" help:"The instruction you'll be given, when it fires, which is all you'll have, so it says everything that's needed, such as Give David the weather in Berlin, and three news headlines, briefly, to schedule, or update" optional:""`
	Schedule  string `json:"schedule,omitempty" help:"When it fires, again and again, as a cron expression, minute hour day month weekday, such as 0 8 * * 1-5 for eight on weekdays, to schedule, or update" optional:""`
	At        string `json:"at,omitempty" help:"When it fires, once, such as 2026-11-06T15:00:00, in the person's time zone, or with an offset, such as 2026-11-06T15:00:00+01:00, to schedule, or update" optional:""`
	Timezone  string `json:"timezone,omitempty" help:"Time zone of the schedule, by its IANA name, such as Europe/Berlin, only when it must stay in that zone, as it otherwise follows the person's" optional:""`
}

// heartbeatOutput is a heartbeat, as a model is told it
type heartbeatOutput struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Prompt   string `json:"prompt"`
	Schedule string `json:"schedule,omitempty"`
	At       string `json:"at,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	Next     string `json:"next,omitempty"`
}

// listOutput is the conversation's heartbeats
type listOutput struct {
	Count      uint              `json:"count"`
	Heartbeats []heartbeatOutput `json:"heartbeats"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - NAMESPACE

func (*Heartbeats) Name() string {
	return Name
}

// Description returns what the heartbeats are
func (*Heartbeats) Description() string {
	return "Prompts which come back to a conversation, once, at a time, or on a schedule, such as reminders, and briefings"
}

// SystemPrompt returns when to schedule heartbeats, and how, and the time
// zone their schedules are in, unless they have their own
func (h *Heartbeats) SystemPrompt() string {
	zone := "UTC, as the person's time zone isn't known"
	if loc, _ := h.personZone(); loc != nil {
		zone = "the person's, " + loc.String()
	}
	now := h.now().In(h.location(""))
	return fmt.Sprintf(heartbeatPrompt, now.Format("Monday, 2 January 2006, 15:04 MST"), zone)
}

// Notify does nothing, as the tools don't change
func (*Heartbeats) Notify(func(eventschema.EventAction, string)) {}

// Tools returns the heartbeats' tool
func (h *Heartbeats) Tools() []llm.Tool {
	return []llm.Tool{&tool{heartbeats: h}}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - TOOL

func (*tool) Name() string {
	return "heartbeats"
}

func (*tool) Description() string {
	return "Heartbeats are instructions which come back to this conversation, once, at a time, or again and again, on a schedule, such as a reminder, or a briefing each morning. Schedule one, list the conversation's heartbeats, update one, or cancel it."
}

func (*tool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[toolInput]()
}

// Run does the operation, saying what it's doing
func (t *tool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req toolInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, llm.ErrBadParameter.With(err.Error())
	}
	session := toolkit.SessionFromContext(ctx).ID()
	switch req.Operation {
	case "schedule":
		if session == uuid.Nil() {
			return nil, llm.ErrBadParameter.With("a heartbeat is scheduled in a conversation, which this isn't")
		}
		at, err := t.at(req.At)
		if err != nil {
			return nil, err
		}
		heartbeat, err := t.heartbeats.Create(ctx, schema.HeartbeatMeta{Title: req.Title, Prompt: req.Prompt, Session: session, Schedule: req.Schedule, At: at, Timezone: req.Timezone})
		if err != nil {
			return nil, err
		}
		toolkit.Logf(ctx, "Scheduling %s", heartbeat.Title)
		return t.output(heartbeat), nil
	case "list":
		toolkit.Logf(ctx, "Looking at what's scheduled")
		list, err := t.heartbeats.List(ctx, schema.HeartbeatListRequest{Session: sessionString(session)})
		if err != nil {
			return nil, err
		}
		result := listOutput{Count: list.Count, Heartbeats: make([]heartbeatOutput, 0, len(list.Body))}
		for _, heartbeat := range list.Body {
			result.Heartbeats = append(result.Heartbeats, t.output(heartbeat))
		}
		return result, nil
	case "update":
		id, err := parseID(req.ID)
		if err != nil {
			return nil, err
		}
		change := schema.HeartbeatRequest{}
		if req.Title != "" {
			change.Title = types.Ptr(req.Title)
		}
		if req.Prompt != "" {
			change.Prompt = types.Ptr(req.Prompt)
		}
		if req.Schedule != "" {
			change.Schedule = types.Ptr(req.Schedule)
		}
		if req.Timezone != "" {
			change.Timezone = types.Ptr(req.Timezone)
		}
		if change.At, err = t.at(req.At); err != nil {
			return nil, err
		}
		heartbeat, err := t.heartbeats.Update(ctx, id, change)
		if err != nil {
			return nil, err
		}
		toolkit.Logf(ctx, "Rescheduling %s", heartbeat.Title)
		return t.output(heartbeat), nil
	case "cancel":
		id, err := parseID(req.ID)
		if err != nil {
			return nil, err
		}
		heartbeat, err := t.heartbeats.Delete(ctx, id)
		if err != nil {
			return nil, err
		}
		toolkit.Logf(ctx, "Cancelling %s", heartbeat.Title)
		return t.output(heartbeat), nil
	}
	return nil, llm.ErrBadParameter.Withf("unknown operation %q, which isn't schedule, list, update or cancel", req.Operation)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// at returns the time, with its offset, or in the person's time zone,
// without one, or nil, when there's none
func (t *tool) at(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if at, err := time.Parse(time.RFC3339, s); err == nil {
		return &at, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if at, err := time.ParseInLocation(layout, s, t.heartbeats.location("")); err == nil {
			return &at, nil
		}
	}
	return nil, llm.ErrBadParameter.Withf("invalid time %q, which must be such as 2026-11-06T15:00:00, or 2026-11-06T15:00:00+01:00", s)
}

// output returns the heartbeat, as a model is told it, with its times in its
// time zone, or the person's
func (t *tool) output(heartbeat schema.Heartbeat) heartbeatOutput {
	loc := t.heartbeats.location(heartbeat.Timezone)
	result := heartbeatOutput{ID: heartbeat.ID.String(), Title: heartbeat.Title, Prompt: heartbeat.Prompt, Schedule: heartbeat.Schedule, Timezone: heartbeat.Timezone}
	if heartbeat.At != nil {
		result.At = heartbeat.At.In(loc).Format(time.RFC3339)
	}
	if heartbeat.Next != nil {
		result.Next = heartbeat.Next.In(loc).Format(time.RFC3339)
	}
	return result
}

// location returns the time zone, by its name, or the person's, when it has
// none, or UTC, when it isn't known
func (h *Heartbeats) location(name string) *time.Location {
	if name != "" {
		if loc, err := schema.LoadLocation(name); err == nil {
			return loc
		}
	} else if loc, _ := h.personZone(); loc != nil {
		return loc
	}
	return time.UTC
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// parseID returns the heartbeat's identifier, or ErrBadParameter when it
// isn't one
func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil(), llm.ErrBadParameter.Withf("invalid heartbeat id %q", s)
	}
	return id, nil
}

// sessionString returns the session's identifier, or none, for the nil UUID
func sessionString(session uuid.UUID) string {
	if session == uuid.Nil() {
		return ""
	}
	return session.String()
}

// heartbeatPrompt is how the model uses heartbeats, with the time, and the
// time zone of their schedules
const heartbeatPrompt = `# Heartbeats

You can schedule heartbeats with the ` + "`heartbeats`" + ` tool: instructions which come back to this conversation later, once, at a time, or again and again, on a schedule. When one fires, you're given its prompt, and you answer it, as a message in this conversation.

- Schedule one when the person asks to be reminded of something, or to have something done at a time, or regularly, such as "remind me to call Mum at six", or "every weekday at eight, tell me the weather and the news".
- Write the prompt as an instruction to yourself, which is all you'll have when it fires, as the conversation may have moved on: say who it's for, what to do, and what to say, such as "Remind David to call his mother", not "remind them about it".
- Use "at", for once, and "schedule", a cron expression, for again and again. Times without an offset are in the time zone below.
- Before scheduling something which may be scheduled already, list the heartbeats, and update one, rather than scheduling it twice. Cancel one when the person no longer wants it.
- Tell the person when it will happen, in their time.

The time is %s. Schedules are in %s, unless they have a time zone of their own.`
