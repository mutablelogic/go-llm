package registry

import (
	"context"
	"log/slog"
	"uuid"
)

// DeleteOldSessions deletes the sessions which haven't changed for longer
// than they're kept, as Run does, and returns how many it deleted
func (r *Registry) DeleteOldSessions(ctx context.Context) int {
	return r.deleteOldSessions(ctx, slog.New(slog.DiscardHandler))
}

// DeleteSessionAt deletes the session, when the serial is its, as the
// sessions which haven't changed for longer than they're kept are
func (r *Registry) DeleteSessionAt(ctx context.Context, id uuid.UUID, serial uint64) error {
	_, err := r.deleteSession(ctx, id, &serial)
	return err
}
