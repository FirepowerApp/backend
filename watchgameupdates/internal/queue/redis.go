package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/tasks"

	"github.com/hibiken/asynq"
)

// RedisQueue implements scheduler.TaskEnqueuer using Redis/Asynq.
type RedisQueue struct {
	client *asynq.Client
}

// NewRedisQueue creates a new RedisQueue from config.
func NewRedisQueue(cfg *config.Config) *RedisQueue {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	return &RedisQueue{client: client}
}

func (q *RedisQueue) Enqueue(_ context.Context, payload models.Payload, deliverAt time.Time) error {
	task, err := tasks.NewWatchGameUpdatesTask(payload)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	opts := []asynq.Option{}
	if delay := time.Until(deliverAt); delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}

	info, err := q.client.Enqueue(task, opts...)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.Info("enqueuing Redis task",
		"game_id", payload.Game.ID,
		"away", payload.Game.AwayTeam.Abbrev,
		"home", payload.Game.HomeTeam.Abbrev,
		"deliver_at", deliverAt.Format(time.RFC3339),
		"task_id", info.ID,
	)

	return nil
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}
