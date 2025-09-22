package services

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
)

type GameDataFetcher interface {
	FetchGameData(gameID string) ([][]string, error)
	GetColumnValue(statColumn string, records [][]string) (string, error)
}

type HTTPGameDataFetcher struct{}

func (f *HTTPGameDataFetcher) GetColumnValue(statColumn string, records [][]string) (string, error) {
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
		return "", fmt.Errorf("could not find one or both columns: homeTeamExpectedGoals, awayTeamExpectedGoals")
	}

	// Print the values from the last row
	fmt.Printf("Stat column value: %s\n", lastRow[statIdx])

	return lastRow[statIdx], nil
}

func (f *HTTPGameDataFetcher) FetchGameData(gameID string) ([][]string, error) {
	fmt.Printf("Fetching new MP data for GameID: %s\n", gameID)

	// Get stats API base URL from environment variable
	statsAPIBaseURL := os.Getenv("STATS_API_BASE_URL")
	if statsAPIBaseURL == "" {
		statsAPIBaseURL = "https://moneypuck.com" // Default production URL
	}

	url := fmt.Sprintf("%s/moneypuck/gameData/20242025/%s.csv", statsAPIBaseURL, gameID)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch game data: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to fetch game data, status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("failed to fetch game data, status code: %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)

	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("Failed to read CSV data: %v", err)
		return nil, err
	}

	return records, nil
}
