package main

import (
	"context"
	"log"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/notification/notifiers"
	"watchgameupdates/internal/queue"
	"watchgameupdates/internal/schedule"
	"watchgameupdates/internal/scheduler"
	"watchgameupdates/internal/season"
)

func main() {
	log.SetFlags(0)

	cfg := config.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Resolve offseason data source. Detection always probes the live NHL
	// API (season.NewDetector's fixed BaseURL), independent of
	// cfg.ScheduleAPIBaseURL — never circular. Emulator routing is gated on
	// APP_ENV=="staging" inside ResolveDataSource, so production always
	// resolves to live regardless of the offseason signal.
	detector := season.NewDetector(cfg.NHLSeasonAPIBaseURL, cfg.SeasonOverride)
	offseason := detector.IsOffseason(ctx)
	dataSource := season.ResolveDataSource(cfg.Env, offseason)
	log.Printf("Offseason detection: offseason=%v, APP_ENV=%s -> DataSource=%s", offseason, cfg.Env, dataSource)

	// Resolve schedule fetch source and team roster for this run. In
	// emulator mode, the scheduler reads from the emulator's schedule
	// endpoint and widens the roster (OFFSEASON_TEAM_FILTER) so the
	// emulator's narrower offseason slate still schedules games — otherwise
	// the normal TEAM_FILTER can filter out every emulator game.
	scheduleBaseURL := cfg.ScheduleAPIBaseURL
	teamFilters := cfg.TeamFilters
	if dataSource == season.DataSourceEmulator {
		scheduleBaseURL = cfg.EmulatorScheduleBaseURL
		teamFilters = cfg.OffseasonTeamFilters
		log.Printf("Offseason emulator mode: schedule source=%s, team filter=%v", scheduleBaseURL, teamFilters)
	}

	// Create schedule fetcher (file-based or HTTP)
	fetcher := schedule.NewScheduleFetcher(cfg.ScheduleFile, scheduleBaseURL)

	// Create queue (cloudtasks or redis, selected by SCHEDULER_QUEUE env var)
	var taskQueue scheduler.TaskEnqueuer
	switch cfg.SchedulerQueue {
	case "redis":
		log.Printf("Scheduler queue: Redis (%s)", cfg.RedisAddress)
		taskQueue = queue.NewRedisQueue(cfg)
	default:
		log.Printf("Scheduler queue: Cloud Tasks")
		ctQueue, err := queue.NewCloudTasksQueue(ctx, cfg)
		if err != nil {
			log.Fatalf("Failed to create Cloud Tasks queue: %v", err)
		}
		taskQueue = ctQueue
	}
	defer taskQueue.Close()

	// Resolve target date
	date := schedule.ResolveTargetDate(cfg.ScheduleDate)

	// Create notification service for scheduler completion summary
	notifService := notifiers.New(cfg.SchedulerNotify)
	defer notifService.Close()

	// Create and run scheduler
	s := scheduler.New(fetcher, taskQueue, cfg.GameMaxDurationHours, cfg.SchedulerNotify, teamFilters, notifService, cfg.IncludeLiveGames, string(dataSource))
	if err := s.Run(ctx, date); err != nil {
		log.Fatalf("Scheduler failed: %v", err)
	}

	log.Println("Scheduler completed successfully")
}
