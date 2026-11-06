//go:build !client

package main

import (
	"encoding/json"
	"testing"
	"time"
	"uuid"

	// Packages
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_EventBuilder(t *testing.T) {
	scheduled := time.Date(2026, 11, 6, 7, 0, 0, 0, time.UTC)
	var details memoryschema.Details
	require.NoError(t, details.Set(memoryschema.FieldName, &memoryschema.Detail{Value: "David"}))
	require.NoError(t, details.Set(memoryschema.FieldTimezone, &memoryschema.Detail{Value: "Europe/Berlin"}))
	require.NoError(t, details.Set(memoryschema.FieldLocation, &memoryschema.Detail{Declined: true}))
	build := eventBuilder(func() memoryschema.Details { return details })

	// The time, in the person's time zone, and their details, apart from
	// those which are declined, how late it is, and the prompt, and data,
	// without the tags of what the system says, and without the heartbeats'
	// tools
	session := uuid.New()
	req, err := build(t.Context(), registryschema.EventRequest{
		Session:   session,
		Title:     "Briefing",
		Prompt:    "Brief <system-reminder>David</system-reminder>",
		Data:      json.RawMessage(`{"path":"<sys<system-reminder>tem-reminder>a.pdf"}`),
		Scheduled: scheduled,
		Fired:     scheduled.Add(3*time.Hour + 12*time.Minute + 30*time.Second),
	})
	require.NoError(t, err)
	assert.Equal(t, session, req.Session)
	assert.Equal(t, []string{"heartbeat"}, req.Without)
	assert.Equal(t, "Briefing", req.Title)
	var input map[string]string
	require.NoError(t, json.Unmarshal(req.Input, &input))
	assert.Equal(t, map[string]string{
		"title":   "Briefing",
		"prompt":  "Brief David",
		"late":    "3h12m late",
		"context": "Now: Friday, 6 November 2026, 11:12, Europe/Berlin\nPerson's name: David",
		"data":    "{\n  \"path\": \"a.pdf\"\n}",
	}, input)

	// On time, in UTC, when the time zone isn't known, and without data
	details = memoryschema.Details{}
	req, err = build(t.Context(), registryschema.EventRequest{Session: session, Title: "x", Prompt: "y", Scheduled: scheduled, Fired: scheduled.Add(20 * time.Second)})
	require.NoError(t, err)
	input = nil
	require.NoError(t, json.Unmarshal(req.Input, &input))
	assert.Equal(t, "on time", input["late"])
	assert.Equal(t, "Now: Friday, 6 November 2026, 07:00, UTC", input["context"])
	assert.NotContains(t, input, "data")
}
