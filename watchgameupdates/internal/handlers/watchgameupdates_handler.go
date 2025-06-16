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
	"watchgameupdates/internal/services"
	"watchgameupdates/internal/tasks"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func WatchGameUpdatesHandler(w http.ResponseWriter, r *http.Request, fetcher services.GameDataFetcher, recomputeTypes map[string]struct{}) {
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

	lastPlay := services.FetchPlayByPlay(payload.GameID)
	homeTeamExpectedGoals := ""
	awayTeamExpectedGoals := ""

	if _, ok := recomputeTypes[lastPlay.TypeDescKey]; ok {
		homeTeamExpectedGoals, err = fetcher.FetchGameData(payload.GameID, "homeTeamExpectedGoals")
		awayTeamExpectedGoals, err = fetcher.FetchGameData(payload.GameID, "awayTeamExpectedGoals")
	}

	if err != nil {
		log.Printf("Failed to fetch game data: %v", err)
		http.Error(w, "Failed to fetch game data", http.StatusInternalServerError)
		return
	}

	if homeTeamExpectedGoals != "" || awayTeamExpectedGoals != "" {
		log.Printf("Got new values for GameID: %s, Home Team Expected Goals: %s, Away Team Expected Goals: %s", payload.GameID, homeTeamExpectedGoals, awayTeamExpectedGoals)

		// Send firebase message
	}

	shouldReschedule := services.ShouldReschedule(payload, lastPlay)
	log.Printf("Last play type: %s, Force reschedule: %v, Should reschedule: %v\n", lastPlay.TypeDescKey, *payload.ForceReschedule, shouldReschedule)

	if shouldReschedule {
		if err := scheduleNextCheck(r.Context(), payload); err != nil {
			log.Printf("Failed to schedule next check: %v", err)
			http.Error(w, "Failed to schedule next check", http.StatusInternalServerError)
			return
		}
	}
}

// scheduleNextCheck creates a Cloud Task to reschedule the next game check
func scheduleNextCheck(ctx context.Context, payload models.Payload) error {
	cfg := config.LoadConfig()

	tasksClient, err := tasks.NewCloudTasksClient(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to create tasks client: %v", err)
	}
	defer tasksClient.Close()

	// Schedule the task for 30 seconds from now (adjust as needed)
	scheduleTime := timestamppb.New(time.Now().Add(30 * time.Second))

	// Reset force reschedule to false
	newForceReschedule := false
	payload.ForceReschedule = &newForceReschedule

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal reschedule payload: %w", err)
	}

	// Configure your queue path - adjust these values for your setup
	projectID := cfg.ProjectID
	location := cfg.LocationID
	queueName := cfg.QueueID

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectID, location, queueName)

	// Create the task
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

	_, err = tasksClient.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create reschedule task: %w", err)
	}

	log.Printf("Successfully scheduled next check task for game %s at %v", payload.GameID, scheduleTime)
	return nil
}
