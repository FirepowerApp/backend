package main

import (
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/logger"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/tasks"

	"github.com/hibiken/asynq"
)

func main() {
	slog.SetDefault(logger.New())

	gameID := flag.String("game", "", "NHL game ID to watch (required)")
	duration := flag.Duration("duration", 12*time.Minute, "Max execution duration")
	delay := flag.Duration("delay", 0, "Delay before first execution")
	notify := flag.Bool("notify", true, "Send Discord notifications")
	flag.Parse()

	if *gameID == "" {
		slog.Error("--game flag is required")
		os.Exit(1)
	}

	cfg := config.LoadConfig()

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer client.Close()

	// ExecutionEnd is intentionally computed from "now" (not "now + delay"). It is
	// a drop-dead wall-clock deadline: even if the first run is delayed, processing
	// stops by this absolute time. To extend the window when using --delay, pass a
	// larger --duration.
	executionEnd := time.Now().Add(*duration).Format(time.RFC3339)
	shouldNotify := *notify
	payload := models.Payload{
		Game:         models.Game{ID: *gameID},
		ExecutionEnd: &executionEnd,
		ShouldNotify: &shouldNotify,
	}

	task, err := tasks.NewWatchGameUpdatesTask(payload)
	if err != nil {
		slog.Error("failed to create task", "error", err)
		os.Exit(1)
	}

	opts := []asynq.Option{}
	if *delay > 0 {
		opts = append(opts, asynq.ProcessIn(*delay))
	}

	info, err := client.Enqueue(task, opts...)
	if err != nil {
		slog.Error("failed to enqueue task", "error", err)
		os.Exit(1)
	}

	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	slog.Info("task enqueued successfully",
		"id", info.ID,
		"queue", info.Queue,
		"game_id", *gameID,
		"payload", string(payloadJSON),
	)
	if *delay > 0 {
		slog.Info("task delayed", "delay", delay.String())
	}
}
