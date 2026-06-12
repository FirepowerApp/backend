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
	_, err := f.FetchAndParseGameData("2025030415", []string{"homeTeamGoals"})

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
	data, err := f.FetchAndParseGameData("2025030415", []string{"homeTeamGoals", "awayTeamGoals"})

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
	_, err := f.FetchAndParseGameData("2025030415", []string{"homeTeamGoals"})

	if err == nil {
		t.Fatal("expected an error for non-200 response, got nil")
	}
	if errors.Is(err, ErrCSVParse) {
		t.Errorf("HTTP error should not be classified as ErrCSVParse, got %v", err)
	}
}
