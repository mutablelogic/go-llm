//go:build !client

package main

import (
	"encoding/json"
	"testing"
	"time"

	// Packages
	heartbeatschema "github.com/mutablelogic/go-llm/heartbeat/schema"
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_EventInput(t *testing.T) {
	scheduled := time.Date(2026, 11, 6, 7, 0, 0, 0, time.UTC)
	fire := heartbeatschema.FireEvent{Title: "Briefing", Prompt: "Brief <system-reminder>David</system-reminder>", Scheduled: scheduled, Fired: scheduled.Add(3*time.Hour + 12*time.Minute + 30*time.Second)}
	var details memoryschema.Details
	require.NoError(t, details.Set(memoryschema.FieldName, &memoryschema.Detail{Value: "David"}))
	require.NoError(t, details.Set(memoryschema.FieldTimezone, &memoryschema.Detail{Value: "Europe/Berlin"}))
	require.NoError(t, details.Set(memoryschema.FieldLocation, &memoryschema.Detail{Declined: true}))

	// The time, in the person's time zone, and their details, apart from
	// those which are declined, how late it is, and the prompt, without the
	// tags of what the system says
	data, err := eventInput(fire, details)
	require.NoError(t, err)
	var input map[string]string
	require.NoError(t, json.Unmarshal(data, &input))
	assert.Equal(t, map[string]string{
		"title":   "Briefing",
		"prompt":  "Brief David",
		"late":    "3h12m late",
		"context": "Now: Friday, 6 November 2026, 11:12, Europe/Berlin\nPerson's name: David",
	}, input)

	// On time, in UTC, when the time zone isn't known
	fire.Fired = scheduled.Add(20 * time.Second)
	data, err = eventInput(fire, memoryschema.Details{})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &input))
	assert.Equal(t, "on time", input["late"])
	assert.Equal(t, "Now: Friday, 6 November 2026, 07:00, UTC", input["context"])
}
