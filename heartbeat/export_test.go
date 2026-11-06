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

// UntilTick returns how long it is until the heartbeats are next looked at
var UntilTick = untilTick
