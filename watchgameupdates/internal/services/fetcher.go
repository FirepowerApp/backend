package services

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
)

// ErrCSVParse classifies a failure to parse the MoneyPuck CSV (e.g. a transient
// malformed row such as a bare quote in an unquoted field). It is distinct from
// HTTP/network failures so callers can choose to retry shortly instead of
// notifying with missing data. Test for it with errors.Is(err, ErrCSVParse).
var ErrCSVParse = errors.New("moneypuck CSV parse error")

type GameDataFetcher interface {
	FetchGameData(gameID string) ([][]string, error)
	FetchAndParseGameData(gameID string, requiredKeys []string) (map[string]string, error)
	GetColumnValue(statColumn string, records [][]string) (string, error)
	GetTeamNames(records [][]string) (homeTeam, awayTeam string, err error)
}

type HTTPGameDataFetcher struct{}

func (f *HTTPGameDataFetcher) GetColumnValue(statColumn string, records [][]string) (string, error) {
	if len(records) == 0 {
		return "", fmt.Errorf("no data records provided")
	}

	header := records[0]
	lastRow := records[len(records)-1]

	// Find the index of the target column
	var statIdx int = -1
	for i, col := range header {
		if col == statColumn {
			statIdx = i
			break
		}
	}

	// Validate that the column was found
	if statIdx == -1 {
		log.Printf("WARNING: Column '%s' not found in CSV data", statColumn)
		log.Printf("DEBUG: Available columns: %v", header)
		return "", fmt.Errorf("column '%s' not found in CSV data", statColumn)
	}

	// Validate index bounds
	if statIdx >= len(lastRow) {
		log.Printf("ERROR: Column index %d out of bounds for row with %d columns", statIdx, len(lastRow))
		return "", fmt.Errorf("column index out of bounds")
	}

	value := lastRow[statIdx]
	log.Printf("INFO: Extracted column '%s' value: %s (from row %d of %d)", statColumn, value, len(records), len(records))

	return value, nil
}

func (f *HTTPGameDataFetcher) GetTeamNames(records [][]string) (homeTeam, awayTeam string, err error) {
	if len(records) < 2 {
		return "", "", fmt.Errorf("insufficient data to extract team names")
	}

	header := records[0]
	lastRow := records[len(records)-1]

	// Find the indexes of the team columns
	var homeTeamIdx, awayTeamIdx int = -1, -1
	for i, col := range header {
		if col == "team" {
			homeTeamIdx = i
		} else if col == "opposingTeam" {
			awayTeamIdx = i
		}
	}

	// If we can't find team/opposingTeam columns, try homeTeam/awayTeam
	if homeTeamIdx == -1 || awayTeamIdx == -1 {
		for i, col := range header {
			if col == "homeTeam" {
				homeTeamIdx = i
			} else if col == "awayTeam" {
				awayTeamIdx = i
			}
		}
	}

	if homeTeamIdx == -1 || awayTeamIdx == -1 {
		// If we still can't find team columns, log available columns for debugging
		log.Printf("Available columns: %v", header)
		return "", "", fmt.Errorf("could not find team name columns in CSV data")
	}

	homeTeam = lastRow[homeTeamIdx]
	awayTeam = lastRow[awayTeamIdx]

	log.Printf("Extracted team names - Home: %s, Away: %s", homeTeam, awayTeam)
	return homeTeam, awayTeam, nil
}

func (f *HTTPGameDataFetcher) FetchGameData(gameID string) ([][]string, error) {
	log.Printf("INFO: Fetching MoneyPuck data for game %s", gameID)

	// Get stats API base URL from environment variable
	statsAPIBaseURL := os.Getenv("STATS_API_BASE_URL")
	if statsAPIBaseURL == "" {
		statsAPIBaseURL = "https://moneypuck.com" // Default production URL
	}

	url := fmt.Sprintf("%s/moneypuck/gameData/20252026/%s.csv", statsAPIBaseURL, gameID)
	log.Printf("DEBUG: Requesting URL: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("ERROR: HTTP request failed for game %s: %v", gameID, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: MoneyPuck API returned status %d for game %s", resp.StatusCode, gameID)
		return nil, fmt.Errorf("failed to fetch game data, status code: %d", resp.StatusCode)
	}

	log.Printf("INFO: Successfully received MoneyPuck data for game %s", gameID)
	reader := csv.NewReader(resp.Body)

	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("ERROR: Failed to parse CSV data for game %s: %v", gameID, err)
		// A *csv.ParseError means MoneyPuck served a malformed file; tag it as
		// ErrCSVParse so callers can retry. Other (I/O) errors pass through as-is.
		var parseErr *csv.ParseError
		if errors.As(err, &parseErr) {
			return nil, fmt.Errorf("%w: %w", ErrCSVParse, err)
		}
		return nil, err
	}

	log.Printf("INFO: Successfully parsed CSV data for game %s - %d total records", gameID, len(records))
	return records, nil
}

func (f *HTTPGameDataFetcher) FetchAndParseGameData(gameID string, requiredKeys []string) (map[string]string, error) {
	records, err := f.FetchGameData(gameID)
	if err != nil {
		return nil, err
	}

	data := make(map[string]string)

	for _, key := range requiredKeys {
		value, err := f.GetColumnValue(key, records)
		if err != nil {
			log.Printf("WARNING: Could not extract value for key '%s': %v", key, err)
			continue
		}
		data[key] = value
	}

	return data, nil
}
