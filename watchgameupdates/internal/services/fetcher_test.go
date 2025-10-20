package services

import (
	"fmt"
	"strconv"
	"testing"
)

// TestMoneyPuckLiveEndpoint tests calling the live MoneyPuck endpoint
// and extracts expected goals and true goals for home/away teams from the last row
func TestMoneyPuckLiveEndpoint(t *testing.T) {
	// Use a specific game ID - test will fail if this game is not found
	gameID := "2025020091"

	// Fetch game data from MoneyPuck - fail immediately if not found
	fetcher := &HTTPGameDataFetcher{}
	t.Logf("Testing MoneyPuck live endpoint for game %s", gameID)
	records, err := fetcher.FetchGameData(gameID)
	if err != nil {
		t.Fatalf("Failed to fetch game data for %s: %v", gameID, err)
	}

	t.Logf("Successfully found data for game %s", gameID)

	if len(records) < 2 {
		t.Fatalf("Insufficient data: expected header + data rows, got %d records", len(records))
	}

	t.Logf("Successfully fetched %d records from MoneyPuck", len(records))

	// Print header to understand available columns (one per line)
	header := records[0]
	t.Logf("Available columns (%d total):", len(header))
	for i, col := range header {
		t.Logf("  [%d]: %s", i, col)
	}

	// Try to extract expected goals data
	var homeExpectedGoals string
	var homeActualGoals string

	// Find expected goals columns - fail if not found
	value, err := fetcher.GetColumnValue("homeTeamExpectedGoals", records)
	if err != nil {
		t.Fatalf("Failed to get expected goals data from column 'homeTeamExpectedGoals': %v", err)
	}
	homeExpectedGoals = value
	t.Logf("Found expected goals in column 'homeTeamExpectedGoals': %s", value)

	// Find actual goals columns - fail if not found
	value, err = fetcher.GetColumnValue("homeTeamGoals", records)
	if err != nil {
		t.Fatalf("Failed to get actual goals data from column 'homeTeamGoals': %v", err)
	}
	homeActualGoals = value
	t.Logf("Found actual goals in column 'homeTeamGoals': %s", value)

	// Print all extracted data
	fmt.Printf("\n=== MoneyPuck Live Endpoint Test Results ===\n")
	fmt.Printf("Game ID: %s\n", gameID)
	fmt.Printf("Data extracted from last row (%d of %d total rows)\n", len(records), len(records))
	fmt.Printf("\n--- Expected Goals (xG) ---\n")
	if homeExpectedGoals != "" {
		if xG, err := strconv.ParseFloat(homeExpectedGoals, 64); err == nil {
			fmt.Printf("Home Expected Goals: %.3f\n", xG)
		} else {
			fmt.Printf("Home Expected Goals: %s (raw)\n", homeExpectedGoals)
		}
	} else {
		fmt.Printf("Home Expected Goals: Not found\n")
	}

	fmt.Printf("\n--- Actual Goals ---\n")
	if homeActualGoals != "" {
		if goals, err := strconv.Atoi(homeActualGoals); err == nil {
			fmt.Printf("Home Actual Goals: %d\n", goals)
		} else {
			fmt.Printf("Home Actual Goals: %s (raw)\n", homeActualGoals)
		}
	} else {
		fmt.Printf("Home Actual Goals: Not found\n")
	}

	fmt.Printf("==========================================\n\n")

	// Basic validation
	if len(records) == 0 {
		t.Error("No records received from MoneyPuck API")
	}

	// Log test completion
	t.Logf("Test completed successfully - extracted data from %d records", len(records))
}

// TestMoneyPuckDataStructure helps understand the actual data structure
func TestMoneyPuckDataStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping data structure test in short mode")
	}

	// Use a specific game ID - test will fail if this game is not found
	gameID := "2025020091"

	fetcher := &HTTPGameDataFetcher{}
	records, err := fetcher.FetchGameData(gameID)
	if err != nil {
		t.Fatalf("Could not fetch data for analysis for game %s: %v", gameID, err)
	}

	if len(records) < 2 {
		t.Skipf("Insufficient data for analysis")
		return
	}

	// Print first few rows for analysis
	fmt.Printf("\n=== MoneyPuck Data Structure Analysis ===\n")
	fmt.Printf("Total records: %d\n", len(records))

	// Print header
	fmt.Printf("\nHeader row:\n")
	for i, col := range records[0] {
		fmt.Printf("  [%d] %s\n", i, col)
	}

	// Print first data row
	if len(records) > 1 {
		fmt.Printf("\nFirst data row:\n")
		for i, val := range records[1] {
			if i < len(records[0]) {
				fmt.Printf("  %s: %s\n", records[0][i], val)
			}
		}
	}

	// Print last data row
	if len(records) > 2 {
		lastRow := records[len(records)-1]
		fmt.Printf("\nLast data row:\n")
		for i, val := range lastRow {
			if i < len(records[0]) {
				fmt.Printf("  %s: %s\n", records[0][i], val)
			}
		}
	}

	fmt.Printf("========================================\n\n")
}
