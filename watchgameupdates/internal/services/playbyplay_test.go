package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

	lastPlay, _ := FetchPlayByPlay("2025030415", "live")

	if lastPlay.TypeDescKey != "goal" {
		t.Errorf("expected FetchPlayByPlay to hit the live server, got play type %q (want %q)", lastPlay.TypeDescKey, "goal")
	}
}

func TestFetchPlayByPlay_EmulatorDataSourceUsesEmulatorBaseURL(t *testing.T) {
	emulator := servePlayByPlay(t)
	deadLive := "http://127.0.0.1:1" // unroutable; must not be hit

	t.Setenv("APP_ENV", "staging")
	t.Setenv("PLAYBYPLAY_API_BASE_URL", deadLive)
	t.Setenv("EMULATOR_PLAYBYPLAY_BASE_URL", emulator.URL)

	lastPlay, _ := FetchPlayByPlay("2025030415", "emulator")

	if lastPlay.TypeDescKey != "goal" {
		t.Errorf("expected FetchPlayByPlay to hit the emulator server, got play type %q (want %q)", lastPlay.TypeDescKey, "goal")
	}
}

func TestFetchPlayByPlay_EmulatorDataSourceOutsideStagingFallsBackToLive(t *testing.T) {
	live := servePlayByPlay(t)
	deadEmulator := "http://127.0.0.1:1" // unroutable; must not be hit

	t.Setenv("APP_ENV", "production")
	t.Setenv("PLAYBYPLAY_API_BASE_URL", live.URL)
	t.Setenv("EMULATOR_PLAYBYPLAY_BASE_URL", deadEmulator)

	lastPlay, _ := FetchPlayByPlay("2025030415", "emulator")

	if lastPlay.TypeDescKey != "goal" {
		t.Errorf("expected the production re-gate (finding 4A) to route to live even with DataSource=emulator, got play type %q", lastPlay.TypeDescKey)
	}
}
