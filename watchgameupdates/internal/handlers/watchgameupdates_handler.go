package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

	log.Printf("Raw body: %s", body)

	// Check execution window using shared logic
	skip, err := services.ShouldSkipExecution(payload)
	if err != nil {
		http.Error(w, "Invalid scheduled_time format", http.StatusBadRequest)
		return
	}
	if skip {
		return
	}

	// Use shared game processor for core logic
	processor := &services.GameProcessor{
		Fetcher:             fetcher,
		NotificationService: notificationService,
	}
	result := processor.ProcessGameUpdate(payload)

	if result.ShouldReschedule {
		if err := scheduleNextCheck(payload); err != nil {
			log.Printf("Failed to schedule next check: %v", err)
			http.Error(w, "Failed to schedule next check", http.StatusInternalServerError)
			return
		}
	}
}

// scheduleNextCheck creates a Cloud Task to reschedule the next game check
func scheduleNextCheck(payload models.Payload) error {
	cfg := config.LoadConfig()

	tasksCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tasksClient, err := tasks.NewCloudTasksClient(tasksCtx, cfg)
	if err != nil {
		log.Fatalf("failed to create tasks client: %v", err)
	}
	defer tasksClient.Close()

	messageInterval := time.Duration(cfg.MessageIntervalSeconds) * time.Second
	scheduleTime := timestamppb.New(time.Now().Add(messageInterval))

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal reschedule payload: %w", err)
	}

	projectID := cfg.ProjectID
	location := cfg.LocationID
	queueName := cfg.QueueID

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectID, location, queueName)

	if payload.ExecutionEnd != nil {
		log.Printf("Max execution time for game %s is set to %s", payload.Game.ID, *payload.ExecutionEnd)
	} else {
		log.Printf("Max execution time for game %s is not set", payload.Game.ID)
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

	log.Printf("Sending task creation request for game %s to Cloud Tasks queue %s", payload.Game.ID, queuePath)

	_, err = tasksClient.CreateTask(tasksCtx, req)
	if err != nil {
		return fmt.Errorf("failed to create reschedule task: %w", err)
	}

	log.Printf("Successfully scheduled next check task for game %s at %v", payload.Game.ID, scheduleTime.AsTime().Format(time.RFC3339))
	return nil
}
