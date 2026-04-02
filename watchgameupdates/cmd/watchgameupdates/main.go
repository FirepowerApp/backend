package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"watchgameupdates/config"
	"watchgameupdates/internal/handlers"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/services"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

func handler(w http.ResponseWriter, r *http.Request) {
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

	// Call the handler
	handlers.WatchGameUpdatesHandler(
		w,
		r,
		fetcher,
		notificationService,
		payload)
}

func main() {
	// Remove timestamp prefix from logs - Docker/structured logging handles timestamps
	log.SetFlags(0)

	cfg := config.LoadConfig()
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

	funcframework.RegisterHTTPFunction("/", handler)
	if err := funcframework.Start("8080"); err != nil {
		log.Fatalf("Failed to start function: %v", err)
	}
}
