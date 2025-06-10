package services

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
)

type GameDataFetcher interface {
	FetchGameData(gameID string, statColumn string) (string, error)
}

type HTTPGameDataFetcher struct{}

func (f *HTTPGameDataFetcher) FetchGameData(gameID string, statColumn string) (string, error) {
	fmt.Printf("Fetching new MP data for GameID: %s\n", gameID)

	url := fmt.Sprintf("https://moneypuck.com/moneypuck/gameData/20242025/%s.csv", gameID)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch game data: %v", err)
		// http.Error(w, "Failed to fetch game data", http.StatusInternalServerError)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to fetch game data, status code: %d", resp.StatusCode)
		// http.Error(w, "Failed to fetch game data", http.StatusInternalServerError)
		return "", fmt.Errorf("failed to fetch game data, status code: %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)

	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("Failed to read CSV data: %v", err)
		// http.Error(w, "Failed to fetch game data", http.StatusInternalServerError)
		return "", err
	}

	header := records[0]
	lastRow := records[len(records)-1]

	// Find the indexes of the target columns
	var statIdx int = -1
	for i, col := range header {
		if col == statColumn {
			statIdx = i
		}
	}

	// Validate that both columns were found
	if statIdx == -1 {
		log.Printf("Could not find column: %s\n", statColumn)
		// http.Error(w, "Failed to fetch game data", http.StatusInternalServerError)
		return "", fmt.Errorf("could not find one or both columns: homeTeamExpectedGoals, awayTeamExpectedGoals")
	}

	// Print the values from the last row
	fmt.Printf("Stat column value: %s\n", lastRow[statIdx])

	return lastRow[statIdx], nil
}
