package main

import (
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"

	"watchgameupdates/config"
	"watchgameupdates/internal/handlers"
	"watchgameupdates/internal/logger"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/notification/liveactivity"
	"watchgameupdates/internal/services"
	"watchgameupdates/internal/tasks"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/hibiken/asynq"
)

// laNotifier is initialized once at startup; nil if LIVEACTIVITY_PUSH_ENABLED is not set.
var laNotifier *liveactivity.LiveActivityNotifier

func init() {
	var err error
	laNotifier, err = liveactivity.New()
	if err != nil {
		slog.Info("LiveActivity notifier not configured (normal if not enabled)", "error", err)
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	fetcher := &services.HTTPGameDataFetcher{}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload models.Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var notificationService *notification.Service
	if payload.ShouldNotify != nil {
		notificationService = notification.NewServiceWithNotificationFlag(*payload.ShouldNotify)
	} else {
		notificationService = notification.NewService()
	}
	if laNotifier != nil {
		notificationService.RegisterNotifier(laNotifier)
	}

	handlers.WatchGameUpdatesHandler(
		w,
		r,
		fetcher,
		notificationService,
		payload)
}

func startHTTPMode(cfg *config.Config) {
	slog.Info("starting in HTTP mode (Cloud Tasks)")
	slog.Info("config loaded",
		"app_env", cfg.Env,
		"gcp_project_id", cfg.ProjectID,
		"gcp_location", cfg.LocationID,
		"cloud_tasks_queue", cfg.QueueID,
		"use_tasks_emulator", cfg.UseEmulator,
		"cloud_tasks_emulator_host", cfg.CloudTasksAddress,
		"handler_host", cfg.HandlerAddress,
		"message_interval_seconds", cfg.MessageIntervalSeconds,
		"period_end_interval_seconds", cfg.PeriodEndIntervalSeconds,
	)

	funcframework.RegisterHTTPFunction("/", httpHandler)
	if err := funcframework.Start("8080"); err != nil {
		slog.Error("failed to start function", "error", err)
		os.Exit(1)
	}
}

func startWorkerMode(cfg *config.Config) {
	slog.Info("starting in worker mode", "redis_address", cfg.RedisAddress)

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	client := asynq.NewClient(redisOpt)
	defer client.Close()

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"default": 1,
		},
	})

	mux := asynq.NewServeMux()
	handler := tasks.NewWatchGameUpdatesHandler(cfg, client)
	mux.HandleFunc(tasks.TypeWatchGameUpdates, handler.ProcessTask)

	slog.Info("asynq worker ready, listening for tasks")

	if err := srv.Run(mux); err != nil {
		slog.Error("failed to start asynq worker", "error", err)
		os.Exit(1)
	}
}

func main() {
	slog.SetDefault(logger.New())

	mode := flag.String("mode", "http", "Run mode: 'http' for Cloud Tasks handler, 'worker' for Redis queue worker")
	flag.Parse()

	cfg := config.LoadConfig()

	switch *mode {
	case "http":
		startHTTPMode(cfg)
	case "worker":
		startWorkerMode(cfg)
	default:
		slog.Error("unknown mode", "mode", *mode)
		os.Exit(1)
	}
}
