package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/services"
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
		// fmt.Printf("Skipping recompute for play type: %s", lastPlay.TypeDescKey)
		homeTeamExpectedGoals, err = fetcher.FetchGameData(payload.GameID, "homeTeamExpectedGoals")
		awayTeamExpectedGoals, err = fetcher.FetchGameData(payload.GameID, "awayTeamExpectedGoals")
	}

	if err != nil {
		log.Printf("Failed to fetch game data: %v", err)
		http.Error(w, "Failed to fetch game data", http.StatusInternalServerError)
		return
	}

	if homeTeamExpectedGoals != "" || awayTeamExpectedGoals != "" {
		log.Printf("Fetched new MP data for GameID: %s, Home Team Expected Goals: %s, Away Team Expected Goals: %s", payload.GameID, homeTeamExpectedGoals, awayTeamExpectedGoals)
	}

	shouldReschedule := services.ShouldReschedule(payload, lastPlay)
	log.Printf("Last play type: %s, Force reschedule: %v, Should reschedule: %v\n", lastPlay.TypeDescKey, *payload.ForceReschedule, shouldReschedule)

	if shouldReschedule {
		// Get appropriate cloud task client and send message
	}
}
