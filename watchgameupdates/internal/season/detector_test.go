package season

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serveSchedule(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func scheduleJSON(preSeasonStartDate string, numberOfGames int) string {
	return fmt.Sprintf(`{"preSeasonStartDate":%q,"numberOfGames":%d}`, preSeasonStartDate, numberOfGames)
}

func TestDetector_IsOffseason_OverrideOffseasonSkipsProbe(t *testing.T) {
	// No server configured — BaseURL points nowhere. If the override didn't
	// short-circuit, this would fail to connect and (via the fail-safe) return
	// false, masking the bug. Assert true to prove the override took effect.
	d := NewDetector("http://127.0.0.1:1", "offseason")
	if !d.IsOffseason(context.Background()) {
		t.Error("expected SEASON_OVERRIDE=offseason to report offseason without probing")
	}
}

func TestDetector_IsOffseason_OverrideInseasonSkipsProbe(t *testing.T) {
	d := NewDetector("http://127.0.0.1:1", "inseason")
	if d.IsOffseason(context.Background()) {
		t.Error("expected SEASON_OVERRIDE=inseason to report in-season without probing")
	}
}

func TestDetector_IsOffseason_TodayBeforePreSeasonAndZeroGames(t *testing.T) {
	today := time.Now().UTC()
	future := today.Add(60 * 24 * time.Hour).Format("2006-01-02")
	srv := serveSchedule(t, scheduleJSON(future, 0), http.StatusOK)

	d := NewDetector(srv.URL, "")
	if !d.IsOffseason(context.Background()) {
		t.Error("expected offseason when today < preSeasonStartDate and numberOfGames == 0")
	}
}

func TestDetector_IsOffseason_TodayOnOrAfterPreSeason(t *testing.T) {
	past := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	srv := serveSchedule(t, scheduleJSON(past, 0), http.StatusOK)

	d := NewDetector(srv.URL, "")
	if d.IsOffseason(context.Background()) {
		t.Error("expected in-season when today >= preSeasonStartDate")
	}
}

func TestDetector_IsOffseason_GuardRejectsNonZeroGamesEvenIfBeforePreSeason(t *testing.T) {
	future := time.Now().UTC().Add(60 * 24 * time.Hour).Format("2006-01-02")
	srv := serveSchedule(t, scheduleJSON(future, 4), http.StatusOK)

	d := NewDetector(srv.URL, "")
	if d.IsOffseason(context.Background()) {
		t.Error("expected in-season when numberOfGames > 0, even if today < preSeasonStartDate")
	}
}

func TestDetector_IsOffseason_ProbeErrorFailsSafeToInSeason(t *testing.T) {
	// Unroutable address triggers a connection error.
	d := NewDetector("http://127.0.0.1:1", "")
	if d.IsOffseason(context.Background()) {
		t.Error("expected probe failure to fail safe to in-season")
	}
}

func TestDetector_IsOffseason_NonOKStatusFailsSafeToInSeason(t *testing.T) {
	srv := serveSchedule(t, `{}`, http.StatusInternalServerError)
	d := NewDetector(srv.URL, "")
	if d.IsOffseason(context.Background()) {
		t.Error("expected non-200 response to fail safe to in-season")
	}
}

func TestDetector_IsOffseason_MalformedJSONFailsSafeToInSeason(t *testing.T) {
	srv := serveSchedule(t, `{not json`, http.StatusOK)
	d := NewDetector(srv.URL, "")
	if d.IsOffseason(context.Background()) {
		t.Error("expected malformed JSON to fail safe to in-season")
	}
}

func TestDetector_IsOffseason_UnparseablePreSeasonDateFailsSafeToInSeason(t *testing.T) {
	srv := serveSchedule(t, scheduleJSON("not-a-date", 0), http.StatusOK)
	d := NewDetector(srv.URL, "")
	if d.IsOffseason(context.Background()) {
		t.Error("expected unparseable preSeasonStartDate to fail safe to in-season")
	}
}

func TestDetector_IsOffseason_MissingPreSeasonDateFailsSafeToInSeason(t *testing.T) {
	srv := serveSchedule(t, `{"numberOfGames":0}`, http.StatusOK)
	d := NewDetector(srv.URL, "")
	if d.IsOffseason(context.Background()) {
		t.Error("expected missing preSeasonStartDate to fail safe to in-season")
	}
}

// TestDetector_IsOffseason_BoundedProbeDoesNotHangCaller verifies the probe
// carries its own short deadline (finding 7A) rather than depending solely on
// the caller's context — a hung NHL API must fail fast, not stall the
// scheduler's full run budget.
func TestDetector_IsOffseason_BoundedProbeDoesNotHangCaller(t *testing.T) {
	// Cleanup order matters: t.Cleanup runs LIFO, and httptest.Server.Close
	// blocks until in-flight handlers return. Registering srv.Close first
	// (so it runs LAST, after the channel unblocks the handler) avoids a
	// deadlock where Close waits forever on a handler that's waiting on this
	// same cleanup to unblock it.
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never responds within the test
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })

	d := NewDetector(srv.URL, "")
	start := time.Now()
	offseason := d.IsOffseason(context.Background())
	elapsed := time.Since(start)

	if offseason {
		t.Error("expected a hung probe to fail safe to in-season")
	}
	if elapsed > probeTimeout+2*time.Second {
		t.Errorf("expected probe to return within its bounded timeout (~%s), took %s", probeTimeout, elapsed)
	}
}

func TestDetector_IsOffseason_UnknownOverrideValueFallsThroughToProbe(t *testing.T) {
	future := time.Now().UTC().Add(60 * 24 * time.Hour).Format("2006-01-02")
	srv := serveSchedule(t, scheduleJSON(future, 0), http.StatusOK)

	d := NewDetector(srv.URL, "bogus")
	if !d.IsOffseason(context.Background()) {
		t.Error("expected an unrecognized override value to fall through to the live probe")
	}
}
