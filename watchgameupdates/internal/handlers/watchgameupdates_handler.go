package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/services"
	"watchgameupdates/internal/tasks"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func WatchGameUpdatesHandler(
	w http.ResponseWriter,
	r *http.Request,
	fetcher services.GameDataFetcher,
	notificationService *notification.Service,
	payload models.Payload) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	slog.Debug("received request body", "game_id", payload.Game.ID, "body", string(body))

	// Check execution window using shared logic
	skip, err := services.ShouldSkipExecution(payload)
	if err != nil {
		http.Error(w, "Invalid scheduled_time format", http.StatusBadRequest)
		return
	}
	if skip {
		return
	}

	// Use shared game processor for core logic.
	processor := &services.GameProcessor{
		Fetcher:             fetcher,
		NotificationService: notificationService,
	}
	result := processor.ProcessGameUpdate(payload)

	if result.ShouldReschedule {
		cfg := config.LoadConfig()
		interval := services.RescheduleInterval(result.LastPlay, result.MaxPeriods, cfg)
		if err := scheduleNextCheck(payload, interval); err != nil {
			slog.Error("failed to schedule next check", "game_id", payload.Game.ID, "error", err)
			http.Error(w, "Failed to schedule next check", http.StatusInternalServerError)
			return
		}
	}
}

// scheduleNextCheck creates a Cloud Task to reschedule the next game check
func scheduleNextCheck(payload models.Payload, interval time.Duration) error {
	cfg := config.LoadConfig()

	tasksCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tasksClient, err := tasks.NewCloudTasksClient(tasksCtx, cfg)
	if err != nil {
		slog.Error("failed to create tasks client", "error", err)
		os.Exit(1)
	}
	defer tasksClient.Close()

	scheduleTime := timestamppb.New(time.Now().Add(interval))

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal reschedule payload: %w", err)
	}

	projectID := cfg.ProjectID
	location := cfg.LocationID
	queueName := cfg.QueueID

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectID, location, queueName)

	if payload.ExecutionEnd != nil {
		slog.Debug("scheduling next check", "game_id", payload.Game.ID, "execution_end", *payload.ExecutionEnd)
	} else {
		slog.Debug("scheduling next check, no execution end set", "game_id", payload.Game.ID)
	}

	task := &taskspb.Task{
		MessageType: &taskspb.Task_HttpRequest{
			HttpRequest: &taskspb.HttpRequest{
				HttpMethod: taskspb.HttpMethod_POST,
				Url:        cfg.HandlerAddress,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: payloadJSON,
			},
		},
		ScheduleTime: scheduleTime,
	}

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task:   task,
	}

	slog.Info("enqueuing next check task", "game_id", payload.Game.ID, "interval_seconds", int(interval.Seconds()), "queue", queuePath)

	_, err = tasksClient.CreateTask(tasksCtx, req)
	if err != nil {
		return fmt.Errorf("failed to create reschedule task: %w", err)
	}

	slog.Info("next check scheduled", "game_id", payload.Game.ID, "scheduled_at", scheduleTime.AsTime().Format(time.RFC3339))
	return nil
}
