package memory

import (
	"fmt"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// namespace is the namespace of the memory tools, whose system prompt is the
// time, from the clock
type namespace struct {
	now func() time.Time
}

var _ toolkit.Namespace = namespace{}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the memory tools
const Name = "memory"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace of the memory tools, which has no tools yet
func New() toolkit.Namespace {
	return namespace{now: time.Now}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (namespace) Name() string {
	return Name
}

// SystemPrompt returns the current date and time, with the day of the week,
// and the time zone, as the clock says, which is the system's, so a model
// knows when it is
func (n namespace) SystemPrompt() string {
	return currentTime(n.now())
}

// Notify does nothing, as the namespace's tools don't change
func (namespace) Notify(func(eventschema.EventAction, string)) {}

// Tools returns no tools
func (namespace) Tools() []llm.Tool {
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// currentTime returns the sentence of the time, with the day of the week, the
// date, the time of day, and the time zone, by its name, such as
// Europe/Berlin, with its abbreviation, when its name is known, or by its
// abbreviation, such as CET, otherwise, and its offset from UTC
func currentTime(now time.Time) string {
	zone, _ := now.Zone()
	if name := now.Location().String(); name != "Local" && name != zone {
		zone = fmt.Sprintf("%s (%s)", name, zone)
	}
	return fmt.Sprintf("The current date and time is %s, in the %s time zone, UTC%s.",
		now.Format("Monday, 2 January 2006, 15:04"), zone, now.Format("-07:00"))
}
