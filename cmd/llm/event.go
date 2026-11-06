//go:build !client

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Packages
	heartbeatschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	reminder "github.com/mutablelogic/go-llm/internal/reminder"
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// eventInput returns the event agent's input for the heartbeat which fired:
// its title, and prompt, how late it is, and what the system says of the
// person, from the memory's details, and the time, in their time zone, or
// UTC, when it isn't known
func eventInput(fire heartbeatschema.FireEvent, details memoryschema.Details) (json.RawMessage, error) {
	loc := time.UTC
	if zone := details.Zone(); zone != nil {
		loc = zone
	}

	// What the system says, one line each
	lines := []string{"Now: " + fire.Fired.In(loc).Format("Monday, 2 January 2006, 15:04") + ", " + loc.String()}
	for _, detail := range []struct {
		field memoryschema.Field
		label string
	}{
		{memoryschema.FieldName, "Person's name"},
		{memoryschema.FieldLocation, "Where they are"},
		{memoryschema.FieldLanguage, "Their language"},
		{memoryschema.FieldAssistant, "Your name"},
	} {
		if value, ok := details.Value(detail.field); ok {
			lines = append(lines, detail.label+": "+reminder.Strip(value))
		}
	}
	return json.Marshal(map[string]string{
		"title":   reminder.Strip(fire.Title),
		"prompt":  reminder.Strip(fire.Prompt),
		"late":    lateness(fire.Scheduled, fire.Fired),
		"context": strings.Join(lines, "\n"),
	})
}

// lateness returns how late the time it fired is, after it was due, in
// words, such as on time, or 3h12m late, to the minute
func lateness(scheduled, fired time.Time) string {
	late := fired.Sub(scheduled).Truncate(time.Minute)
	if late < time.Minute {
		return "on time"
	}
	return fmt.Sprintf("%s late", strings.TrimSuffix(late.String(), "0s"))
}
