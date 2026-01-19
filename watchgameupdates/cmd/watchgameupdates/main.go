package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
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

	funcframework.RegisterHTTPFunction("/", handler)
	if err := funcframework.Start("8080"); err != nil {
		log.Fatalf("Failed to start function: %v", err)
	}
}
