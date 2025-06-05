package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// TaskPayload represents the payload structure for the task.
// It contains a single field, GameID, which is a string.
// This structure is used to pass data from the HTTP request to the task processing logic.
// GameID is the string identifier from the NHL API for the game that the task will process.
// The GameID is obtained from the daily schedule activity, and passed to the task through a cloud task.
type TaskPayload struct {
	GameID string `json:"game_id"`
}

func PollHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload TaskPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	fmt.Printf("Polling for GameID: %s\n", payload.GameID)

	// Query the below URL for game data
	// https://moneypuck.com/moneypuck/gameData/20242025/2024030325.csv
	url := "https://moneypuck.com/moneypuck/gameData/20242025/2024030325.csv"
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to fetch game data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Failed to fetch game data, status code: %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)

	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV data: %v", err)
	}

	header := records[0]
	lastRow := records[len(records)-1]

	// Find the indexes of the target columns
	var homeIdx, awayIdx int = -1, -1
	for i, col := range header {
		if col == "homeTeamExpectedGoals" {
			homeIdx = i
		} else if col == "awayTeamExpectedGoals" {
			awayIdx = i
		}
	}

	// Validate that both columns were found
	if homeIdx == -1 || awayIdx == -1 {
		log.Fatalf("Could not find one or both columns: homeTeamExpectedGoals, awayTeamExpectedGoals")
	}

	// Print the values from the last row
	fmt.Printf("homeTeamExpectedGoals: %s\n", lastRow[homeIdx])
	fmt.Printf("awayTeamExpectedGoals: %s\n", lastRow[awayIdx])

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

	// eventResp := fetchEventData(payload.GameID)
	// fmt.Printf("Fetched event data: %+v\n", eventResp)

	// if eventResp.Status == "done" {
	// 	fmt.Println("Polling complete: game is done.")
	// 	return
	// }

	// if eventResp.Team == payload.TeamID && eventResp.Type == "shot" {
	// 	goals := fetchGoalCount(payload.GameID)
	// 	fmt.Printf("Send to Firebase topic '%s' with goals: %d\n", payload.TeamID, goals)
	// } else {
	// 	fmt.Println("No relevant update for polling.")
	// }

}
