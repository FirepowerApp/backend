package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"

	"watchgameupdates/config"
	"watchgameupdates/internal/handlers"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/notification/notifiers"
	"watchgameupdates/internal/services"
	"watchgameupdates/internal/tasks"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/hibiken/asynq"
)

func makeHTTPHandler(svc *notification.Service) http.HandlerFunc {
	fetcher := &services.HTTPGameDataFetcher{}
	return func(w http.ResponseWriter, r *http.Request) {
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

		shouldNotify := payload.ShouldNotify == nil || *payload.ShouldNotify
		handlers.WatchGameUpdatesHandler(
			w,
			r,
			fetcher,
			svc.WithShouldNotify(shouldNotify),
			payload)
	}
}

func startHTTPMode(cfg *config.Config) {
	log.Println("Starting in HTTP mode (Cloud Tasks)")

	// Build the notification service once so JWT signers and HTTP connections are
	// reused across requests. Per-request shouldNotify is applied via WithShouldNotify.
	sharedNotifService := notifiers.New(true)

	log.Printf("Config loaded:")
	log.Printf("  APP_ENV:                    %s", cfg.Env)
	log.Printf("  GCP_PROJECT_ID:             %s", cfg.ProjectID)
	log.Printf("  GCP_LOCATION:               %s", cfg.LocationID)
	log.Printf("  CLOUD_TASKS_QUEUE:          %s", cfg.QueueID)
	log.Printf("  USE_TASKS_EMULATOR:         %v", cfg.UseEmulator)
	log.Printf("  CLOUD_TASKS_EMULATOR_HOST:  %s", cfg.CloudTasksAddress)
	log.Printf("  HANDLER_HOST:               %s", cfg.HandlerAddress)
	log.Printf("  MESSAGE_INTERVAL_SECONDS:   %d", cfg.MessageIntervalSeconds)
	log.Printf("  PERIOD_END_INTERVAL_SECONDS:%d", cfg.PeriodEndIntervalSeconds)

	funcframework.RegisterHTTPFunction("/", makeHTTPHandler(sharedNotifService))
	if err := funcframework.Start("8080"); err != nil {
		log.Fatalf("Failed to start function: %v", err)
	}
}

func startWorkerMode(cfg *config.Config) {
	log.Printf("Starting in worker mode (Redis at %s)", cfg.RedisAddress)

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

	log.Printf("Asynq worker ready, listening for tasks...")

	if err := srv.Run(mux); err != nil {
		log.Fatalf("Failed to start asynq worker: %v", err)
	}
}

func main() {
	// Remove timestamp prefix from logs - Docker/structured logging handles timestamps
	log.SetFlags(0)

	mode := flag.String("mode", "http", "Run mode: 'http' for Cloud Tasks handler, 'worker' for Redis queue worker")
	flag.Parse()

	cfg := config.LoadConfig()

	switch *mode {
	case "http":
		startHTTPMode(cfg)
	case "worker":
		startWorkerMode(cfg)
	default:
		log.Fatalf("Unknown mode: %s. Use 'http' or 'worker'.", *mode)
	}
}
