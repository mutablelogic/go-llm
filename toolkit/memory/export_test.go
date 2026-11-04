package memory

import (
	"time"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
)

// NewWithClock returns the namespace of the memory tools, with the clock
func NewWithClock(now func() time.Time) toolkit.Namespace {
	return namespace{now: now}
}
