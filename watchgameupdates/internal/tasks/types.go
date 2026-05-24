package tasks

import (
	"encoding/json"
	"fmt"

	"watchgameupdates/internal/models"

	"github.com/hibiken/asynq"
)

const (
	TypeWatchGameUpdates = "game:watch_updates"
)

// NewWatchGameUpdatesTask creates a new asynq task from a game payload.
func NewWatchGameUpdatesTask(payload models.Payload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return asynq.NewTask(TypeWatchGameUpdates, data), nil
}

// ParseWatchGameUpdatesPayload deserializes a payload from an asynq task.
func ParseWatchGameUpdatesPayload(t *asynq.Task) (models.Payload, error) {
	var payload models.Payload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return payload, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return payload, nil
}
