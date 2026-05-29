package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/services"

	"github.com/hibiken/asynq"
)

// TaskEnqueuer abstracts the ability to enqueue tasks, enabling test mocking.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// WatchGameUpdatesHandler processes game update tasks from the Redis queue.
type WatchGameUpdatesHandler struct {
	cfg      *config.Config
	enqueuer TaskEnqueuer
}

func NewWatchGameUpdatesHandler(cfg *config.Config, enqueuer TaskEnqueuer) *WatchGameUpdatesHandler {
	return &WatchGameUpdatesHandler{cfg: cfg, enqueuer: enqueuer}
}

func (h *WatchGameUpdatesHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	payload, err := ParseWatchGameUpdatesPayload(t)
	if err != nil {
		// Poison pill: malformed JSON will never succeed on retry. Drop the task.
		slog.Error("dropping task with invalid payload", "error", err)
		return nil
	}

	slog.Info("processing task", "game_id", payload.Game.ID)

	// Check execution window
	skip, err := services.ShouldSkipExecution(payload)
	if err != nil {
		// Poison pill: malformed ExecutionEnd will never parse on retry. Drop the task.
		slog.Error("dropping task with invalid execution window", "game_id", payload.Game.ID, "error", err)
		return nil
	}
	if skip {
		slog.Info("execution window expired, task complete", "game_id", payload.Game.ID)
		return nil
	}

	// Build processor with fresh notification service
	fetcher := &services.HTTPGameDataFetcher{}
	var notificationService *notification.Service
	if payload.ShouldNotify != nil {
		notificationService = notification.NewServiceWithNotificationFlag(*payload.ShouldNotify)
	} else {
		notificationService = notification.NewService()
	}
	defer notificationService.Close()

	processor := &services.GameProcessor{
		Fetcher:             fetcher,
		NotificationService: notificationService,
	}

	result := processor.ProcessGameUpdate(payload)

	if result.ShouldReschedule {
		if err := h.scheduleNextCheck(payload); err != nil {
			return fmt.Errorf("failed to schedule next check for game %s: %w", payload.Game.ID, err)
		}
	}

	return nil
}

func (h *WatchGameUpdatesHandler) scheduleNextCheck(payload models.Payload) error {
	task, err := NewWatchGameUpdatesTask(payload)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	interval := time.Duration(h.cfg.MessageIntervalSeconds) * time.Second
	info, err := h.enqueuer.Enqueue(task, asynq.ProcessIn(interval))
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.Info("next check scheduled", "game_id", payload.Game.ID, "task_id", info.ID, "interval", interval.String())
	return nil
}
