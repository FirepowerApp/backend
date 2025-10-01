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

func WatchGameUpdatesHandler(w http.ResponseWriter, r *http.Request, fetcher services.GameDataFetcher, recomputeTypes map[string]struct{}, notifier Notifier) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	log.Printf("Raw body: %s", body)

	var payload models.Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if payload.ExecutionEnd != nil {
		executionEnd, err := time.Parse(time.RFC3339, *payload.ExecutionEnd)
		if err != nil {
			http.Error(w, "Invalid scheduled_time format", http.StatusBadRequest)
			return
		}

		if time.Now().After(executionEnd) {
			log.Printf("Current time is after execution end (%s). Skipping execution.", executionEnd.Format(time.RFC3339))
			return
		}
	} else {
		log.Println("Max execution time not set, proceeding without time check.")
	}

	lastPlay := services.FetchPlayByPlay(payload.GameID)
	homeTeamExpectedGoals := ""
	awayTeamExpectedGoals := ""

	if _, ok := recomputeTypes[lastPlay.TypeDescKey]; ok {
		records, err := fetcher.FetchGameData(payload.GameID)
		if err != nil {
			log.Printf("Failed to fetch game data: %v", err)
			// http.Error(w, "Failed to fetch game data", http.StatusInternalServerError)
			// return
			homeTeamExpectedGoals = "-1"
			awayTeamExpectedGoals = "-1"
		} else {
			homeTeamExpectedGoals, err = fetcher.GetColumnValue("homeTeamExpectedGoals", records)
			awayTeamExpectedGoals, err = fetcher.GetColumnValue("awayTeamExpectedGoals", records)
		}

		if homeTeamExpectedGoals != "" || awayTeamExpectedGoals != "" {
			log.Printf("Got new values for GameID: %s, Home Team Expected Goals: %s, Away Team Expected Goals: %s", payload.GameID, homeTeamExpectedGoals, awayTeamExpectedGoals)

			// Extract team names for the notification
			homeTeam, awayTeam, err := fetcher.GetTeamNames(records)
			if err != nil {
				log.Printf("Failed to extract team names: %v", err)
				// Continue with generic team names
				homeTeam = "Home Team"
				awayTeam = "Away Team"
			}

			// Send notification using the provided notifier
			sendExpectedGoalsNotification(notifier, homeTeam, awayTeam, homeTeamExpectedGoals, awayTeamExpectedGoals)
		}
	}

	shouldReschedule := services.ShouldReschedule(payload, lastPlay)
	log.Printf("Last play type: %s, Should reschedule: %v\n", lastPlay.TypeDescKey, shouldReschedule)

	if shouldReschedule {
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

	scheduleTime := timestamppb.New(time.Now().Add(60 * time.Second))

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal reschedule payload: %w", err)
	}

	// Configure your queue path - adjust these values for your setup
	projectID := cfg.ProjectID
	location := cfg.LocationID
	queueName := cfg.QueueID

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectID, location, queueName)

	if payload.ExecutionEnd != nil {
		log.Printf("Max execution time for game %s is set to %s", payload.GameID, *payload.ExecutionEnd)
	} else {
		log.Printf("Max execution time for game %s is not set", payload.GameID)
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

	log.Printf("Sending task creation request for game %s to Cloud Tasks queue %s", payload.GameID, queuePath)

	_, err = tasksClient.CreateTask(tasksCtx, req)
	if err != nil {
		return fmt.Errorf("failed to create reschedule task: %w", err)
	}

	log.Printf("Successfully scheduled next check task for game %s at %v", payload.GameID, scheduleTime.AsTime().Format(time.RFC3339))
	return nil
}
