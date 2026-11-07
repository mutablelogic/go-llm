package registry

import (
	"context"
	"log/slog"
)

// DeleteOldSessions deletes the sessions which haven't changed for longer
// than they're kept, as Run does, and returns how many it deleted
func (r *Registry) DeleteOldSessions(ctx context.Context) int {
	return r.deleteOldSessions(ctx, slog.New(slog.DiscardHandler))
}
