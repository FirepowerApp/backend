package main

import (
	"log"
	"net/http"
	"watchgameupdates/internal/handlers"
	"watchgameupdates/internal/services"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

func handler(w http.ResponseWriter, r *http.Request) {
	// Initialize dependencies
	recomputeTypes := map[string]struct{}{
		"blocked-shot": {},
		"missed-shot":  {},
		"shot-on-goal": {},
		"goal":         {},
	}
	fetcher := &services.HTTPGameDataFetcher{}

	// Call the handler
	handlers.WatchGameUpdatesHandler(w, r, fetcher, recomputeTypes)
}

func main() {
	funcframework.RegisterHTTPFunction("/", handler)
	if err := funcframework.Start("8080"); err != nil {
		log.Fatalf("Failed to start function: %v", err)
	}
}
