package queue

import (
	"context"
	"time"

	"watchgameupdates/internal/models"
)

// GameTaskQueue is the interface for enqueuing game tracking tasks.
// Implementations exist for Cloud Tasks (now) and Redis (future).
type GameTaskQueue interface {
	// Enqueue schedules a game tracking task for delivery at the specified time.
	Enqueue(ctx context.Context, payload models.Payload, deliverAt time.Time) error
	Close() error
}
