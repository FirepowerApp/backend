package schedule

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHTTPScheduleFetcher_FetchSchedule(t *testing.T) {
	resp := ScheduleResponse{
		GameWeek: []GameWeekDay{
			{
				Date: "2025-10-07",
				Games: []ScheduleGame{
					{ID: 2025020001, GameDate: "2025-10-07", GameState: "OFF"},
				},
			},
			{
				Date: "2025-10-08",
				Games: []ScheduleGame{
					{ID: 2025020010, GameDate: "2025-10-08", GameState: "FUT", StartTimeUTC: "2025-10-08T23:00:00Z"},
					{ID: 2025020011, GameDate: "2025-10-08", GameState: "FUT", StartTimeUTC: "2025-10-09T00:00:00Z"},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/schedule/2025-10-08" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	fetcher := NewHTTPScheduleFetcher(server.URL)
	games, err := fetcher.FetchSchedule(context.Background(), "2025-10-08")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(games) != 2 {
		t.Fatalf("expected 2 games for 2025-10-08, got %d", len(games))
	}

	if games[0].ID != 2025020010 {
		t.Errorf("expected game ID 2025020010, got %d", games[0].ID)
	}
}

func TestHTTPScheduleFetcher_NoGamesForDate(t *testing.T) {
	resp := ScheduleResponse{
		GameWeek: []GameWeekDay{
			{
				Date:  "2025-10-07",
				Games: []ScheduleGame{{ID: 1, GameState: "OFF"}},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	fetcher := NewHTTPScheduleFetcher(server.URL)
	games, err := fetcher.FetchSchedule(context.Background(), "2025-10-08")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(games) != 0 {
		t.Fatalf("expected 0 games, got %d", len(games))
	}
}

func TestFileScheduleFetcher_FetchSchedule(t *testing.T) {
	resp := ScheduleResponse{
		GameWeek: []GameWeekDay{
			{
				Date: "2025-10-08",
				Games: []ScheduleGame{
					{ID: 2025020010, GameDate: "2025-10-08", GameState: "FUT", StartTimeUTC: "2025-10-08T23:00:00Z"},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "schedule.json")
	data, _ := json.Marshal(resp)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fetcher := &FileScheduleFetcher{FilePath: filePath}
	games, err := fetcher.FetchSchedule(context.Background(), "2025-10-08")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}

	if games[0].ID != 2025020010 {
		t.Errorf("expected game ID 2025020010, got %d", games[0].ID)
	}
}

func TestResolveTargetDate_Override(t *testing.T) {
	date := ResolveTargetDate("2025-12-25")
	if date != "2025-12-25" {
		t.Errorf("expected 2025-12-25, got %s", date)
	}
}

func TestResolveTargetDate_Default(t *testing.T) {
	date := ResolveTargetDate("")
	if date == "" {
		t.Error("expected non-empty date")
	}
	// Should be in YYYY-MM-DD format
	if len(date) != 10 {
		t.Errorf("expected date in YYYY-MM-DD format, got %s", date)
	}
}
