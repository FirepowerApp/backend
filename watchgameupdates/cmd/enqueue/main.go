package main

import (
	"encoding/json"
	"flag"
	"log"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/tasks"

	"github.com/hibiken/asynq"
)

func main() {
	gameID := flag.String("game", "", "NHL game ID to watch (required)")
	duration := flag.Duration("duration", 12*time.Minute, "Max execution duration")
	delay := flag.Duration("delay", 0, "Delay before first execution")
	notify := flag.Bool("notify", true, "Send Discord notifications")
	flag.Parse()

	if *gameID == "" {
		log.Fatal("--game flag is required")
	}

	cfg := config.LoadConfig()

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer client.Close()

	executionEnd := time.Now().Add(*duration).Format(time.RFC3339)
	payload := models.Payload{
		Game:         models.Game{ID: *gameID},
		ExecutionEnd: &executionEnd,
		ShouldNotify: notify,
	}

	task, err := tasks.NewWatchGameUpdatesTask(payload)
	if err != nil {
		log.Fatalf("Failed to create task: %v", err)
	}

	opts := []asynq.Option{}
	if *delay > 0 {
		opts = append(opts, asynq.ProcessIn(*delay))
	}

	info, err := client.Enqueue(task, opts...)
	if err != nil {
		log.Fatalf("Failed to enqueue task: %v", err)
	}

	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	log.Printf("Task enqueued successfully:")
	log.Printf("  ID:       %s", info.ID)
	log.Printf("  Queue:    %s", info.Queue)
	log.Printf("  Game:     %s", *gameID)
	log.Printf("  Payload:  %s", payloadJSON)
	if *delay > 0 {
		log.Printf("  Delay:    %v", *delay)
	}
}
