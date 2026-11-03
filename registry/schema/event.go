package schema

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The events of a stream of the registry's events: each event, and that
// the server is stopping, before the stream ends, so a client knows it
// ended as there will be no more events
const (
	EventName     = "event"
	EventShutdown = "shutdown"
)
