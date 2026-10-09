package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// servePlayByPlay starts a test server returning a single play with the given
// typeDescKey, wired to both PLAYBYPLAY_API_BASE_URL (live) and
// EMULATOR_PLAYBYPLAY_BASE_URL, and returns the server so the caller can
// assert on which env var was actually consulted.
func servePlayByPlay(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"plays":[{"typeDescKey":"goal","periodDescriptor":{"number":1,"periodType":"REG"},"timeRemaining":"12:00"}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchPlayByPlay_LiveDataSourceUsesLiveBaseURL(t *testing.T) {
	live := servePlayByPlay(t)
	deadEmulator := "http://127.0.0.1:1" // unroutable; must not be hit

	t.Setenv("APP_ENV", "staging")
	t.Setenv("PLAYBYPLAY_API_BASE_URL", live.URL)
	t.Setenv("EMULATOR_PLAYBYPLAY_BASE_URL", deadEmulator)

	resp, err := FetchPlayByPlay("2025030415", "live")

	if err != nil || lastPlayType(resp) != "goal" {
		t.Errorf("expected FetchPlayByPlay to hit the live server, got play type %q, err %v (want %q)", lastPlayType(resp), err, "goal")
	}
}

func TestFetchPlayByPlay_EmulatorDataSourceUsesEmulatorBaseURL(t *testing.T) {
	emulator := servePlayByPlay(t)
	deadLive := "http://127.0.0.1:1" // unroutable; must not be hit

	t.Setenv("APP_ENV", "staging")
	t.Setenv("PLAYBYPLAY_API_BASE_URL", deadLive)
	t.Setenv("EMULATOR_PLAYBYPLAY_BASE_URL", emulator.URL)

	resp, err := FetchPlayByPlay("2025030415", "emulator")

	if err != nil || lastPlayType(resp) != "goal" {
		t.Errorf("expected FetchPlayByPlay to hit the emulator server, got play type %q, err %v (want %q)", lastPlayType(resp), err, "goal")
	}
}

func TestFetchPlayByPlay_EmulatorDataSourceOutsideStagingFallsBackToLive(t *testing.T) {
	live := servePlayByPlay(t)
	deadEmulator := "http://127.0.0.1:1" // unroutable; must not be hit

	t.Setenv("APP_ENV", "production")
	t.Setenv("PLAYBYPLAY_API_BASE_URL", live.URL)
	t.Setenv("EMULATOR_PLAYBYPLAY_BASE_URL", deadEmulator)

	resp, err := FetchPlayByPlay("2025030415", "emulator")

	if err != nil || lastPlayType(resp) != "goal" {
		t.Errorf("expected the production re-gate (finding 4A) to route to live even with DataSource=emulator, got play type %q, err %v", lastPlayType(resp), err)
	}
}

func TestFetchPlayByPlay_ReadsTopLevelGameFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixtureOT))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PLAYBYPLAY_API_BASE_URL", srv.URL)

	resp, err := FetchPlayByPlay("2026020026", "live")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GameState != "OFF" || resp.GameType != 2 {
		t.Errorf("expected gameState OFF / gameType 2, got %q / %d", resp.GameState, resp.GameType)
	}
	if resp.HomeTeam.Score != 6 || resp.AwayTeam.Score != 5 {
		t.Errorf("expected 6-5, got %d-%d", resp.HomeTeam.Score, resp.AwayTeam.Score)
	}
	if resp.Clock.TimeRemaining != "03:27" || resp.Clock.InIntermission {
		t.Errorf("unexpected clock %+v", resp.Clock)
	}
}

func TestFetchPlayByPlay_FailuresAreReturnedNotSwallowed(t *testing.T) {
	testCases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"Non200", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }},
		// This used to panic and kill the request instead of surfacing an error.
		{"MalformedJSON", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"plays":[`)) }},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)
			t.Setenv("PLAYBYPLAY_API_BASE_URL", srv.URL)

			resp, err := FetchPlayByPlay("2025030415", "live")
			if err == nil || resp != nil {
				t.Errorf("expected an error and nil response, got resp=%v err=%v", resp, err)
			}
		})
	}
}

// A hung upstream must fail, not park the game check forever. Both fetches go
// through httpClient; shortening its deadline keeps the test fast, and a
// regression to a bare http.Get would block here until the test times out.
func TestUpstreamFetchesAreBoundedByTimeout(t *testing.T) {
	if httpClient.Timeout != upstreamHTTPTimeout {
		t.Fatalf("expected httpClient timeout %v, got %v", upstreamHTTPTimeout, httpClient.Timeout)
	}
	orig := httpClient.Timeout
	httpClient.Timeout = 100 * time.Millisecond
	t.Cleanup(func() { httpClient.Timeout = orig })

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	t.Setenv("PLAYBYPLAY_API_BASE_URL", srv.URL)
	t.Setenv("STATS_API_BASE_URL", srv.URL)

	fetches := map[string]func() error{
		"playbyplay": func() error { _, err := FetchPlayByPlay("2025030415", "live"); return err },
		"moneypuck":  func() error { _, err := (&HTTPGameDataFetcher{}).FetchGameData("2025030415", "live"); return err },
	}
	for name, fetch := range fetches {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			if err := fetch(); err == nil {
				t.Error("expected a timeout error from a hung upstream")
			}
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Errorf("fetch took %v; the deadline was not applied", elapsed)
			}
		})
	}
}
