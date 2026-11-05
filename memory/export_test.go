package memory

import "time"

// SetClock sets the clock of the memory, for its system prompt
func SetClock(m *Memory, now func() time.Time) {
	m.now = now
}
