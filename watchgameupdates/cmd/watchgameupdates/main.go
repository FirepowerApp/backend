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
	"watchgameupdates/internal/services"
	"watchgameupdates/internal/tasks"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/hibiken/asynq"
)

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

	handlers.WatchGameUpdatesHandler(
		w,
		r,
		fetcher,
		notificationService,
		payload)
}

func startHTTPMode() {
	log.Println("Starting in HTTP mode (Cloud Tasks)")

	funcframework.RegisterHTTPFunction("/", httpHandler)
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
		startHTTPMode()
	case "worker":
		startWorkerMode(cfg)
	default:
		log.Fatalf("Unknown mode: %s. Use 'http' or 'worker'.", *mode)
	}
}
