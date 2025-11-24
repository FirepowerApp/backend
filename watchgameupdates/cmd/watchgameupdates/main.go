package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"watchgameupdates/config"
	"watchgameupdates/internal/handlers"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/services"

	"github.com/hibiken/asynq"
)

func main() {
	cfg := config.LoadConfig()

	// Create WaitGroup to coordinate graceful shutdown
	var wg sync.WaitGroup

	// Channel for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server in goroutine
	wg.Add(1)
	httpServer := startHTTPServer(&wg, cfg)

	// Start Asynq worker in goroutine
	wg.Add(1)
	asynqServer := startAsynqWorker(&wg, cfg)

	log.Println("Backend started successfully:")
	log.Printf("  - HTTP server listening on :8080")
	log.Printf("  - Asynq worker connected to %s", cfg.RedisAddress)
	log.Printf("  - Worker concurrency: 10")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, gracefully stopping...")

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Shutdown Asynq worker
	asynqServer.Shutdown()

	// Wait for both to finish
	wg.Wait()
	log.Println("Shutdown complete")
}

// startHTTPServer starts the HTTP server for health checks and external triggers
func startHTTPServer(wg *sync.WaitGroup, cfg *config.Config) *http.Server {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Main handler endpoint (for backward compatibility or external triggers)
	mux.HandleFunc("/", httpHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		defer wg.Done()
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	return server
}

// httpHandler handles HTTP requests (optional, for external triggers)
func httpHandler(w http.ResponseWriter, r *http.Request) {
	var payload models.Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("Invalid payload: %v", err), http.StatusBadRequest)
		return
	}

	// Process immediately or enqueue for async processing
	if err := processGameUpdate(payload); err != nil {
		http.Error(w, fmt.Sprintf("Processing error: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Processing started"))
}

// startAsynqWorker starts the Asynq worker that processes messages from Redis
func startAsynqWorker(wg *sync.WaitGroup, cfg *config.Config) *asynq.Server {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
	}

	// Create server with concurrent processing
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			// Process up to 10 games concurrently
			Concurrency: 10,

			// Retry failed tasks with exponential backoff
			RetryDelayFunc: func(n int, err error, task *asynq.Task) time.Duration {
				// Retry after 30s, 1m, 2m, 4m...
				return time.Duration(30*(1<<uint(n))) * time.Second
			},

			// Maximum 3 retries
			MaxRetry: 3,

			// Queues with priority (default queue has priority 1)
			Queues: map[string]int{
				"critical": 6, // Highest priority
				"default":  3,
				"low":      1,
			},

			// Error handler for failed tasks
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Printf("Task failed [ID=%s]: %v", task.Type(), err)
			}),

			// Log level
			LogLevel: asynq.InfoLevel,
		},
	)

	// Create multiplexer for routing tasks to handlers
	mux := asynq.NewServeMux()

	// Register handler for game check tasks
	mux.HandleFunc("game:check", handleGameCheckTask)

	// Start server in goroutine
	go func() {
		defer wg.Done()
		if err := srv.Run(mux); err != nil {
			log.Fatalf("Asynq server error: %v", err)
		}
	}()

	return srv
}

// handleGameCheckTask processes a single game check task
// This runs concurrently - Asynq manages the goroutine pool
func handleGameCheckTask(ctx context.Context, task *asynq.Task) error {
	// Parse payload
	var payload models.Payload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		// Return error to trigger retry
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("Processing game check for game %s (Task ID: %s)", payload.Game.ID, task.Type())

	// Process the game update
	if err := processGameUpdate(payload); err != nil {
		// This will trigger retry based on RetryDelayFunc
		return fmt.Errorf("failed to process game %s: %w", payload.Game.ID, err)
	}

	log.Printf("Successfully processed game %s", payload.Game.ID)
	return nil
}

// processGameUpdate contains the core business logic
// This is extracted so it can be called from both HTTP handler and Asynq worker
func processGameUpdate(payload models.Payload) error {
	// Initialize dependencies
	recomputeTypes := map[string]struct{}{
		"blocked-shot": {},
		"missed-shot":  {},
		"shot-on-goal": {},
		"goal":         {},
	}
	fetcher := &services.HTTPGameDataFetcher{}

	// Initialize notifier
	notifier, err := handlers.NewDiscordNotifier()
	if err != nil {
		log.Printf("Warning: Failed to create notifier: %v", err)
		notifier = nil
	}

	// Check if execution window has passed
	if payload.ExecutionEnd != nil {
		executionEnd, err := time.Parse(time.RFC3339, *payload.ExecutionEnd)
		if err != nil {
			return fmt.Errorf("invalid execution_end format: %w", err)
		}
		if time.Now().After(executionEnd) {
			log.Printf("Execution window expired for game %s, skipping", payload.Game.ID)
			return nil
		}
	}

	// Fetch latest play-by-play data
	lastPlay := services.FetchPlayByPlay(payload.Game.ID)
	if lastPlay == nil {
		return fmt.Errorf("failed to fetch play-by-play data")
	}

	log.Printf("Game %s - Last play: %s", payload.Game.ID, lastPlay.TypeDescKey)

	// Check if we need to fetch xG data
	if _, shouldRecompute := recomputeTypes[lastPlay.TypeDescKey]; shouldRecompute {
		log.Printf("Fetching xG data for game %s", payload.Game.ID)
		gameData := fetcher.FetchGameData(payload.Game.ID)

		// Send notification if we have a notifier
		if notifier != nil && gameData != nil {
			handlers.SendGameUpdateNotification(notifier, payload.Game, *gameData, lastPlay.TypeDescKey)
		}
	}

	// Check if we should reschedule
	shouldReschedule := services.ShouldReschedule(payload, *lastPlay)
	log.Printf("Game %s - Should reschedule: %t", payload.Game.ID, shouldReschedule)

	if shouldReschedule {
		// Schedule next check
		return handlers.ScheduleNextCheck(payload)
	}

	log.Printf("Game %s monitoring complete", payload.Game.ID)
	return nil
}
