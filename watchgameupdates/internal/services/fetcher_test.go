package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// serveCSV starts a test server returning the given body and points the
// MoneyPuck fetcher at it via STATS_API_BASE_URL. The returned cleanup is
// registered with t.Cleanup.
func serveCSV(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(body))
	}))
	t.Setenv("STATS_API_BASE_URL", srv.URL)
	t.Cleanup(srv.Close)
}

func TestFetchAndParseGameData_MalformedCSVIsClassifiedAsParseError(t *testing.T) {
	// A bare " in an unquoted field — the exact failure MoneyPuck served during
	// the live game. encoding/csv rejects it with *csv.ParseError.
	serveCSV(t, "id,homeTeamGoals,eventDescriptionRaw\n"+
		"1,2,Tkachuk 6'2\" wrister\n")

	f := &HTTPGameDataFetcher{}
	_, err := f.FetchAndParseGameData("2025030415", []string{"homeTeamGoals"}, "")

	if err == nil {
		t.Fatal("expected an error for malformed CSV, got nil")
	}
	if !errors.Is(err, ErrCSVParse) {
		t.Errorf("expected error to match ErrCSVParse, got %v", err)
	}
}

func TestFetchAndParseGameData_ValidCSVIsNotParseError(t *testing.T) {
	serveCSV(t, "id,homeTeamGoals,awayTeamGoals\n"+
		"1,2,1\n")

	f := &HTTPGameDataFetcher{}
	data, err := f.FetchAndParseGameData("2025030415", []string{"homeTeamGoals", "awayTeamGoals"}, "")

	if err != nil {
		t.Fatalf("unexpected error for valid CSV: %v", err)
	}
	if data["homeTeamGoals"] != "2" || data["awayTeamGoals"] != "1" {
		t.Errorf("unexpected extracted values: %v", data)
	}
}

func TestFetchAndParseGameData_HTTPErrorIsNotClassifiedAsParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Setenv("STATS_API_BASE_URL", srv.URL)
	defer srv.Close()

	f := &HTTPGameDataFetcher{}
	_, err := f.FetchAndParseGameData("2025030415", []string{"homeTeamGoals"}, "")

	if err == nil {
		t.Fatal("expected an error for non-200 response, got nil")
	}
	if errors.Is(err, ErrCSVParse) {
		t.Errorf("HTTP error should not be classified as ErrCSVParse, got %v", err)
	}
}

func TestFetchGameData_EmulatorDataSourceUsesEmulatorBaseURL(t *testing.T) {
	emulator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("id,homeTeamGoals\n1,3\n"))
	}))
	defer emulator.Close()

	t.Setenv("APP_ENV", "staging")
	t.Setenv("STATS_API_BASE_URL", "http://127.0.0.1:1") // unroutable; must not be hit
	t.Setenv("EMULATOR_STATS_BASE_URL", emulator.URL)

	f := &HTTPGameDataFetcher{}
	records, err := f.FetchGameData("2025030415", "emulator")

	if err != nil {
		t.Fatalf("expected FetchGameData to hit the emulator server, got error: %v", err)
	}
	if len(records) != 2 || records[1][1] != "3" {
		t.Errorf("unexpected records from emulator server: %v", records)
	}
}

func TestFetchGameData_EmulatorDataSourceOutsideStagingFallsBackToLive(t *testing.T) {
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("id,homeTeamGoals\n1,7\n"))
	}))
	defer live.Close()

	t.Setenv("APP_ENV", "production")
	t.Setenv("STATS_API_BASE_URL", live.URL)
	t.Setenv("EMULATOR_STATS_BASE_URL", "http://127.0.0.1:1") // unroutable; must not be hit

	f := &HTTPGameDataFetcher{}
	records, err := f.FetchGameData("2025030415", "emulator")

	if err != nil {
		t.Fatalf("expected the production re-gate (finding 4A) to route to live, got error: %v", err)
	}
	if len(records) != 2 || records[1][1] != "7" {
		t.Errorf("unexpected records: %v", records)
	}
}
