package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// ScheduleFetcher fetches the NHL schedule for a given date.
type ScheduleFetcher interface {
	FetchSchedule(ctx context.Context, date string) ([]ScheduleGame, error)
}

// HTTPScheduleFetcher fetches the schedule from the live NHL API.
type HTTPScheduleFetcher struct {
	BaseURL string
}

func (f *HTTPScheduleFetcher) FetchSchedule(ctx context.Context, date string) ([]ScheduleGame, error) {
	url := fmt.Sprintf("%s/v1/schedule/%s", f.BaseURL, date)
	log.Printf("Fetching NHL schedule from %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NHL API returned status %d", resp.StatusCode)
	}

	var scheduleResp ScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&scheduleResp); err != nil {
		return nil, fmt.Errorf("failed to decode schedule response: %w", err)
	}

	return filterGamesByDate(scheduleResp, date), nil
}

// filterGamesByDate returns only games matching the target date from the gameWeek response.
func filterGamesByDate(resp ScheduleResponse, date string) []ScheduleGame {
	for _, day := range resp.GameWeek {
		if day.Date == date {
			return day.Games
		}
	}
	return nil
}

// NewHTTPScheduleFetcher creates an HTTPScheduleFetcher.
// If baseURL is empty, it defaults to the live NHL API.
func NewHTTPScheduleFetcher(baseURL string) *HTTPScheduleFetcher {
	if baseURL == "" {
		baseURL = "https://api-web.nhle.com"
	}
	return &HTTPScheduleFetcher{BaseURL: baseURL}
}

// NewScheduleFetcher creates the appropriate ScheduleFetcher based on configuration.
// If scheduleFile is non-empty, returns a file-based fetcher; otherwise returns an HTTP fetcher.
func NewScheduleFetcher(scheduleFile, baseURL string) ScheduleFetcher {
	if scheduleFile != "" {
		return &FileScheduleFetcher{FilePath: scheduleFile}
	}
	return NewHTTPScheduleFetcher(baseURL)
}

// FetchScheduleFromFile reads a schedule from a JSON file and returns the parsed response.
func FetchScheduleFromFile(filePath string) (*ScheduleResponse, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schedule file %s: %w", filePath, err)
	}

	var resp ScheduleResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse schedule file: %w", err)
	}

	return &resp, nil
}

// ResolveTargetDate returns the target date for schedule fetching.
// If dateOverride is non-empty, it is returned as-is; otherwise today's date in UTC is used.
func ResolveTargetDate(dateOverride string) string {
	if dateOverride != "" {
		return dateOverride
	}
	return time.Now().UTC().Format("2006-01-02")
}
