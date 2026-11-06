//go:build !client

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// Packages
	heartbeatschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// reminderTag is the tag of the block of what the system says, which is
// taken out of what the system doesn't say, so it can't be faked
const reminderTag = "system-reminder"

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
			lines = append(lines, detail.label+": "+withoutReminder(value))
		}
	}
	return json.Marshal(map[string]string{
		"title":   withoutReminder(fire.Title),
		"prompt":  withoutReminder(fire.Prompt),
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

// withoutReminder returns the text without the tags of the block of what the
// system says, as what the person, or a model, wrote, so it can't fake one
func withoutReminder(text string) string {
	for _, tag := range []string{"<" + reminderTag + ">", "</" + reminderTag + ">"} {
		text = strings.ReplaceAll(text, tag, "")
	}
	return text
}
