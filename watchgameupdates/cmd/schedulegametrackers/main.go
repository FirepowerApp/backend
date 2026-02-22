package main

import (
	"context"
	"log"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/queue"
	"watchgameupdates/internal/schedule"
	"watchgameupdates/internal/scheduler"
)

func main() {
	log.SetFlags(0)

	cfg := config.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create schedule fetcher (file-based or HTTP)
	fetcher := schedule.NewScheduleFetcher(cfg.ScheduleFile, cfg.ScheduleAPIBaseURL)

	// Create queue
	taskQueue, err := queue.NewCloudTasksQueue(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create task queue: %v", err)
	}
	defer taskQueue.Close()

	// Resolve target date
	date := schedule.ResolveTargetDate(cfg.ScheduleDate)

	// Create and run scheduler
	s := scheduler.New(fetcher, taskQueue, cfg.GameMaxDurationHours, cfg.SchedulerNotify)
	if err := s.Run(ctx, date); err != nil {
		log.Fatalf("Scheduler failed: %v", err)
	}

	log.Println("Scheduler completed successfully")
}
