package schema

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Event struct {
	Event     string `json:"event"`
	Listeners uint   `json:"listener_count"`
}
