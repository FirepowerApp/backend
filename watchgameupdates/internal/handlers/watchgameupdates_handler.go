package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"watchgameupdates/internal/models"
	"watchgameupdates/internal/services"
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

	lastPlay := services.FetchPlayByPlay(payload.Game.ID)
	homeTeamExpectedGoals := ""
	awayTeamExpectedGoals := ""

	if _, ok := recomputeTypes[lastPlay.TypeDescKey]; ok {
		log.Printf("Processing play type '%s' for game %s - fetching MoneyPuck data", lastPlay.TypeDescKey, payload.Game.ID)

		records, err := fetcher.FetchGameData(payload.Game.ID)
		if err != nil {
			log.Printf("ERROR: Failed to fetch MoneyPuck data for game %s: %v", payload.Game.ID, err)
			homeTeamExpectedGoals = "-1"
			awayTeamExpectedGoals = "-1"
		} else {
			// Log CSV structure for debugging
			if len(records) > 0 {
				log.Printf("MoneyPuck CSV structure - Columns: %d, Rows: %d", len(records[0]), len(records))
				log.Printf("Available columns: %v", records[0])
				if len(records) > 1 {
					log.Printf("Sample data row: %v", records[len(records)-1])
				}
			} else {
				log.Printf("WARNING: No data rows returned from MoneyPuck for game %s", payload.Game.ID)
			}

			homeTeamExpectedGoals, err = fetcher.GetColumnValue("homeTeamExpectedGoals", records)
			if err != nil {
				log.Printf("WARNING: Could not extract homeTeamExpectedGoals: %v", err)
			}

			awayTeamExpectedGoals, err = fetcher.GetColumnValue("awayTeamExpectedGoals", records)
			if err != nil {
				log.Printf("WARNING: Could not extract awayTeamExpectedGoals: %v", err)
			}
		}

		if homeTeamExpectedGoals != "" || awayTeamExpectedGoals != "" {
			log.Printf("Got new values for GameID: %s, Home Team Expected Goals: %s, Away Team Expected Goals: %s", payload.Game.ID, homeTeamExpectedGoals, awayTeamExpectedGoals)

			// Get team names from the payload instead of trying to extract from MoneyPuck data
			homeTeam := payload.Game.HomeTeam.CommonName["default"]
			awayTeam := payload.Game.AwayTeam.CommonName["default"]
			if homeTeam == "" {
				homeTeam = "Home Team"
			}
			if awayTeam == "" {
				awayTeam = "Away Team"
			}

			// Extract current scores from MoneyPuck data
			homeTeamGoals, _ := fetcher.GetColumnValue("homeTeamGoals", records)
			awayTeamGoals, _ := fetcher.GetColumnValue("awayTeamGoals", records)

			// Send notification using the provided notifier
			sendExpectedGoalsNotification(notifier, homeTeam, awayTeam, homeTeamExpectedGoals, awayTeamExpectedGoals, homeTeamGoals, awayTeamGoals)
		}
	}

	shouldReschedule := services.ShouldReschedule(payload, lastPlay)
	log.Printf("Last play type: %s, Should reschedule: %v\n", lastPlay.TypeDescKey, shouldReschedule)

	if shouldReschedule {
		// Use the new Asynq-based scheduler
		if err := ScheduleNextCheck(payload); err != nil {
			log.Printf("Failed to schedule next check: %v", err)
			http.Error(w, "Failed to schedule next check", http.StatusInternalServerError)
			return
		}
	}
}
