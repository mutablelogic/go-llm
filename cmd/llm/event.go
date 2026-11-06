//go:build !client

package main

import (
	"context"
	"time"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/heartbeat"
	reminder "github.com/mutablelogic/go-llm/internal/reminder"
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// eventBuilder returns how an event is run, whether a heartbeat which fired,
// or one which the API is given: with the event agent's input, which has
// what the system says of the person, from the memory's details, and the
// time, in their time zone, or UTC, when it isn't known, and without the
// heartbeats' tools, so an event doesn't schedule events
func eventBuilder(details func() memoryschema.Details) registry.EventBuilder {
	return func(_ context.Context, req registryschema.EventRequest) (registryschema.RunEventRequest, error) {
		input, err := req.Input(eventContext(req.Fired, details()))
		if err != nil {
			return registryschema.RunEventRequest{}, err
		}
		return registryschema.RunEventRequest{
			Session:   req.Session,
			Input:     input,
			Without:   []string{heartbeat.Name},
			Title:     req.Title,
			Scheduled: req.Scheduled,
			Fired:     req.Fired,
		}, nil
	}
}

// eventContext returns what the system says of the person, and the time, one
// line each: the time it fired, in their time zone, or UTC, when it isn't
// known, and their details, apart from those which are declined, without
// the tags of what the system says
func eventContext(fired time.Time, details memoryschema.Details) []string {
	loc := time.UTC
	if zone := details.Zone(); zone != nil {
		loc = zone
	}
	lines := []string{"Now: " + fired.In(loc).Format("Monday, 2 January 2006, 15:04") + ", " + loc.String()}
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
	return lines
}
