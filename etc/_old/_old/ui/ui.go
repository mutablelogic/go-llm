package ui

import "context"

//////////////////////////////////////////////////////////////////////////////
// TYPES

type UI interface {
	// Run the runloop for the UI
	Run(ctx context.Context) error

	// Send a system message
	SysPrint(format string, args ...interface{}) error
}
