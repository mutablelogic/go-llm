package heartbeat

import (
	"context"
	"time"
)

// SetClock sets the clock of the heartbeats
func SetClock(h *Heartbeats, now func() time.Time) {
	h.now = now
}

// Tick fires the heartbeats which are due
func Tick(ctx context.Context, h *Heartbeats) {
	h.tick(ctx, nil)
}
