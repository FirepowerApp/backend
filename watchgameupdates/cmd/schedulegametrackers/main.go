package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/logger"
	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/queue"
	"watchgameupdates/internal/schedule"
	"watchgameupdates/internal/scheduler"
)

func main() {
	slog.SetDefault(logger.New())

	cfg := config.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create schedule fetcher (file-based or HTTP)
	fetcher := schedule.NewScheduleFetcher(cfg.ScheduleFile, cfg.ScheduleAPIBaseURL)

	// Create queue (cloudtasks or redis, selected by SCHEDULER_QUEUE env var)
	var taskQueue scheduler.TaskEnqueuer
	switch cfg.SchedulerQueue {
	case "redis":
		slog.Info("scheduler queue: Redis", "redis_address", cfg.RedisAddress)
		taskQueue = queue.NewRedisQueue(cfg)
	default:
		slog.Info("scheduler queue: Cloud Tasks")
		ctQueue, err := queue.NewCloudTasksQueue(ctx, cfg)
		if err != nil {
			slog.Error("failed to create Cloud Tasks queue", "error", err)
			os.Exit(1)
		}
		taskQueue = ctQueue
	}
	defer taskQueue.Close()

	// Resolve target date
	date := schedule.ResolveTargetDate(cfg.ScheduleDate)

	// Create notification service for scheduler completion summary
	notifService := notification.NewServiceWithNotificationFlag(cfg.SchedulerNotify)
	defer notifService.Close()

	// Create and run scheduler
	s := scheduler.New(fetcher, taskQueue, cfg.GameMaxDurationHours, cfg.SchedulerNotify, cfg.TeamFilter, notifService, cfg.IncludeLiveGames)
	if err := s.Run(ctx, date); err != nil {
		slog.Error("scheduler failed", "error", err)
		os.Exit(1)
	}

	slog.Info("scheduler completed successfully")
}
