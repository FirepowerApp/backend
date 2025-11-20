package main

import (
	"log"
	"net/http"
	"watchgameupdates/internal/handlers"
	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/services"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fetcher := &services.HTTPGameDataFetcher{}

	notificationService := notification.NewService()

	// Call the handler
	handlers.WatchGameUpdatesHandler(w, r, fetcher, notificationService)
}

func main() {
	funcframework.RegisterHTTPFunction("/", handler)
	if err := funcframework.Start("8080"); err != nil {
		log.Fatalf("Failed to start function: %v", err)
	}
}
