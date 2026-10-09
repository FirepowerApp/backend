package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
)

// recordingNotifier satisfies notification.Notifier and records what it is
// handed, so a test can assert both that the error paths never reach the
// notifiers and what the data map held when a push did go out. FormatMessage is
// called from the service's goroutine, so reads wait for it.
type recordingNotifier struct {
	sent atomic.Bool
	keys []string // required keys; defaults to just homeTeamGoals
	mu   sync.Mutex
	data map[string]string
}

func (n *recordingNotifier) GetRequiredDataKeys() []string {
	if n.keys != nil {
		return n.keys
	}
	return []string{"homeTeamGoals"}
}

func (n *recordingNotifier) FormatMessage(req notification.NotificationRequest) string {
	n.mu.Lock()
	n.data = req.Data
	n.mu.Unlock()
	n.sent.Store(true)
	return ""
}

// pushed waits for the push and returns the data map the notifier was handed.
func (n *recordingNotifier) pushed(t *testing.T) map[string]string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !n.sent.Load() {
		if time.Now().After(deadline) {
			t.Fatal("expected a push, none arrived")
		}
		time.Sleep(5 * time.Millisecond)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data
}

// assertNoPush gives the service's goroutine time to run before checking.
func (n *recordingNotifier) assertNoPush(t *testing.T) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	if n.sent.Load() {
		t.Error("expected no push")
	}
}

func (n *recordingNotifier) SendNotification(ctx context.Context, message string) (<-chan notification.NotificationResult, error) {
	n.sent.Store(true)
	ch := make(chan notification.NotificationResult, 1)
	ch <- notification.NotificationResult{Success: true, ID: "test"}
	close(ch)
	return ch, nil
}

func (n *recordingNotifier) Close() error { return nil }

// TestProcessGameUpdate_CSVParseErrorSkipsNotifyAndRetries drives the full
// ProcessGameUpdate path against a play-by-play server that reports a period-end
// and a MoneyPuck server that returns a malformed CSV. The processor must send
// nothing and signal a short retry.
func TestProcessGameUpdate_CSVParseErrorSkipsNotifyAndRetries(t *testing.T) {
	// Play-by-play: last play is a period-end (a recompute type → triggers fetch).
	pbp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"plays":[{"typeDescKey":"period-end","periodDescriptor":{"number":1,"periodType":"REG"},"timeRemaining":"00:00"}]}`))
	}))
	defer pbp.Close()
	t.Setenv("PLAYBYPLAY_API_BASE_URL", pbp.URL)

	// MoneyPuck: malformed CSV (bare quote in an unquoted field).
	serveCSV(t, "id,homeTeamGoals\n1,Tkachuk 6'2\" wrister\n")

	notifier := &recordingNotifier{}
	svc := notification.NewServiceWithNotificationFlag(true)
	svc.RegisterNotifier(notifier)

	gp := &GameProcessor{Fetcher: &HTTPGameDataFetcher{}, NotificationService: svc}

	result := gp.ProcessGameUpdate(models.Payload{
		Game: models.Game{
			ID:       "2025030415",
			HomeTeam: models.Team{Abbrev: "CAR", CommonName: map[string]string{"default": "Hurricanes"}},
			AwayTeam: models.Team{Abbrev: "FLA", CommonName: map[string]string{"default": "Panthers"}},
		},
	})

	if notifier.sent.Load() {
		t.Error("expected no notification to be sent on a CSV parse error")
	}
	if !result.RetryAfterDataError {
		t.Error("expected RetryAfterDataError=true on a CSV parse error")
	}
	if !result.ShouldReschedule {
		t.Error("expected ShouldReschedule=true so the check is retried")
	}
}

// TestProcessGameUpdate_ValidCSVNotifies is the control: a well-formed CSV must
// still reach the notifier and must not request the parse-error retry.
func TestProcessGameUpdate_ValidCSVNotifies(t *testing.T) {
	servePBP(t, liveSecondPeriod)
	serveCSV(t, "id,homeTeamGoals\n1,2\n")

	notifier := &recordingNotifier{}
	gp, payload := processWith(notifier)

	result := gp.ProcessGameUpdate(payload)

	if result.RetryAfterDataError {
		t.Error("expected RetryAfterDataError=false for a valid CSV")
	}
	notifier.pushed(t)
}

// A response without the game-level fields (the shape the staging game data
// emulator serves today) leaves nothing true to say. gameState would be empty,
// which iOS renders as "Pregame", so no push goes out. Update the emulator to
// emit gameState/clock/team scores before relying on emulator-mode pushes.
func TestProcessGameUpdate_ResponseWithoutGameFieldsSendsNothing(t *testing.T) {
	servePBP(t, `{"plays":[{"typeDescKey":"goal","periodDescriptor":{"number":1,"periodType":"REG"},"timeRemaining":"12:00"}]}`)
	serveCSV(t, xgCSV)
	notifier := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(notifier)

	result := gp.ProcessGameUpdate(payload)

	notifier.assertNoPush(t)
	if !result.ShouldReschedule {
		t.Error("expected to keep polling")
	}
}

// TestProcessGameUpdate_ForwardsDataSourceToPlayByPlayFetch drives
// ProcessGameUpdate end-to-end with distinct live and emulator play-by-play
// servers to prove payload.DataSource is actually threaded through to the
// fetch call — not just preserved across JSON round-trips (that's covered
// separately by TestProcessTask_RescheduledTaskPreservesDataSource). If
// gameprocessor.go ever stopped passing payload.DataSource to
// FetchPlayByPlay, this test would catch it; the reschedule test would not.
func TestProcessGameUpdate_ForwardsDataSourceToPlayByPlayFetch(t *testing.T) {
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("live PBP server should not be hit when DataSource=emulator and APP_ENV=staging")
	}))
	defer live.Close()

	// "faceoff" is not in ProcessGameUpdate's recompute-types set, so the
	// MoneyPuck stats fetch is skipped and this test only needs to stub PBP.
	emulator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"plays":[{"typeDescKey":"faceoff","periodDescriptor":{"number":1,"periodType":"REG"},"timeRemaining":"10:00"}]}`))
	}))
	defer emulator.Close()

	t.Setenv("APP_ENV", "staging")
	t.Setenv("PLAYBYPLAY_API_BASE_URL", live.URL)
	t.Setenv("EMULATOR_PLAYBYPLAY_BASE_URL", emulator.URL)

	notifier := &recordingNotifier{}
	svc := notification.NewServiceWithNotificationFlag(true)
	svc.RegisterNotifier(notifier)

	gp := &GameProcessor{Fetcher: &HTTPGameDataFetcher{}, NotificationService: svc}

	result := gp.ProcessGameUpdate(models.Payload{
		Game:       models.Game{ID: "2025030415"},
		DataSource: "emulator",
	})

	if got := lastPlayType(result.Response); got != "faceoff" {
		t.Errorf("expected ProcessGameUpdate to route to the emulator PBP server, got play type %q", got)
	}
}

// --- Single-source-of-truth tests: state, clock and score come from the NHL
// play-by-play document; MoneyPuck contributes expected goals only. ---

const liveSecondPeriod = `{"gameType":2,"gameState":"LIVE","periodDescriptor":{"number":2,"periodType":"REG"},` +
	`"clock":{"timeRemaining":"06:56","inIntermission":false},"homeTeam":{"score":3},"awayTeam":{"score":2},` +
	`"plays":[{"typeDescKey":"goal","periodDescriptor":{"number":2,"periodType":"REG"},"timeRemaining":"07:10"}]}`

var allPushKeys = []string{
	"homeTeamGoals", "awayTeamGoals", "homeTeamExpectedGoals", "awayTeamExpectedGoals",
	"gameState", "lastPlayType", "homeTeamAbbrev", "awayTeamAbbrev",
}

const xgCSV = "id,homeTeamExpectedGoals,awayTeamExpectedGoals\n1,2.5,1.5\n"

func servePBP(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PLAYBYPLAY_API_BASE_URL", srv.URL)
}

func processWith(n *recordingNotifier) (*GameProcessor, models.Payload) {
	svc := notification.NewServiceWithNotificationFlag(true)
	svc.RegisterNotifier(n)
	return &GameProcessor{Fetcher: &HTTPGameDataFetcher{}, NotificationService: svc}, models.Payload{
		Game: models.Game{
			ID:       "2026020026",
			HomeTeam: models.Team{Abbrev: "PIT", CommonName: map[string]string{"default": "Penguins"}},
			AwayTeam: models.Team{Abbrev: "MTL", CommonName: map[string]string{"default": "Canadiens"}},
		},
	}
}

// The reported bug, replayed from a real capture: an OT period-end on a goal
// used to push the final score beside "03:27 left, OT" and then wait 20 minutes.
func TestProcessGameUpdate_OvertimeFixtureReportsFinal(t *testing.T) {
	servePBP(t, fixtureOT)
	serveCSV(t, xgCSV)
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)

	result := gp.ProcessGameUpdate(payload)
	data := n.pushed(t)

	if data["gameState"] != "Final" {
		t.Errorf("expected gameState Final, got %q", data["gameState"])
	}
	if data["homeTeamGoals"] != "6" || data["awayTeamGoals"] != "5" {
		t.Errorf("expected 6-5, got %s-%s", data["homeTeamGoals"], data["awayTeamGoals"])
	}
	if result.ShouldReschedule {
		t.Error("a finished game must not be rescheduled")
	}
	// Every key the processor takes off MoneyPuck's plate must still reach the notifier.
	for _, k := range allPushKeys {
		if data[k] == "" {
			t.Errorf("required key %q is empty in the pushed data", k)
		}
	}
}

// NHL's score already includes the shootout decider; MoneyPuck's does not, and
// is ignored. Used to be patched up by AdjustScoreForShootout.
func TestProcessGameUpdate_ShootoutFixtureUsesNHLScore(t *testing.T) {
	servePBP(t, fixtureSO)
	serveCSV(t, "id,homeTeamGoals,awayTeamGoals,homeTeamExpectedGoals,awayTeamExpectedGoals,homeTeamShootOutGoals,awayTeamShootOutGoals\n1,4,4,2.5,1.5,2,1\n")
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)

	gp.ProcessGameUpdate(payload)
	data := n.pushed(t)

	if data["homeTeamGoals"] != "5" || data["awayTeamGoals"] != "4" {
		t.Errorf("expected CAR 5, PIT 4 (decider included), got %s-%s", data["homeTeamGoals"], data["awayTeamGoals"])
	}
	if data["gameState"] != "Final" {
		t.Errorf("expected Final, got %q", data["gameState"])
	}
}

// A MoneyPuck 500 used to push 0-0 with an empty gameState ("Pregame" on iOS)
// mid-game, and assigning into the nil map it left behind would panic.
func TestProcessGameUpdate_MoneyPuckHTTPErrorStillPushesRealScore(t *testing.T) {
	servePBP(t, liveSecondPeriod)
	mp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(mp.Close)
	t.Setenv("STATS_API_BASE_URL", mp.URL)
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)

	result := gp.ProcessGameUpdate(payload)
	data := n.pushed(t)

	if data["homeTeamGoals"] != "3" || data["awayTeamGoals"] != "2" {
		t.Errorf("expected the real 3-2 score, got %s-%s", data["homeTeamGoals"], data["awayTeamGoals"])
	}
	if data["gameState"] != "06:56 left, 2nd period" {
		t.Errorf("unexpected gameState %q", data["gameState"])
	}
	if result.RetryAfterDataError || !result.ShouldReschedule {
		t.Errorf("expected a normal reschedule, got %+v", result)
	}
}

// With no play-by-play there is nothing true to say: send nothing, retry soon.
func TestProcessGameUpdate_PlayByPlayFailureSendsNothingAndRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PLAYBYPLAY_API_BASE_URL", srv.URL)
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)

	result := gp.ProcessGameUpdate(payload)

	n.assertNoPush(t)
	if !result.RetryAfterDataError || !result.ShouldReschedule {
		t.Errorf("expected a short retry, got %+v", result)
	}
	if result.Response != nil {
		t.Error("expected a nil Response on a failed fetch")
	}
}

func TestProcessGameUpdate_PlayByPlayFailurePastExecutionEndDoesNotRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PLAYBYPLAY_API_BASE_URL", srv.URL)
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	payload.ExecutionEnd = &past

	if result := gp.ProcessGameUpdate(payload); result.ShouldReschedule {
		t.Error("a failed fetch must still honor the execution window")
	}
}

// A live response whose plays have not populated yet: no recompute type, so no push.
func TestProcessGameUpdate_LiveWithNoPlaysSendsNothing(t *testing.T) {
	servePBP(t, `{"gameType":2,"gameState":"LIVE","periodDescriptor":{"number":1,"periodType":"REG"},`+
		`"clock":{"timeRemaining":"20:00","inIntermission":false},"homeTeam":{"score":0},"awayTeam":{"score":0},"plays":[]}`)
	serveCSV(t, xgCSV)
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)

	result := gp.ProcessGameUpdate(payload)

	n.assertNoPush(t)
	if !result.ShouldReschedule {
		t.Error("expected to keep polling a game that has not started")
	}
}

// gameState can flip to OFF before the last play is a recompute type. This is the
// last cycle, so the final push must still go out.
func TestProcessGameUpdate_FinalGameAlwaysGetsFinalPush(t *testing.T) {
	servePBP(t, `{"gameType":2,"gameState":"OFF","periodDescriptor":{"number":3,"periodType":"REG"},`+
		`"clock":{"timeRemaining":"00:00","inIntermission":false},"homeTeam":{"score":4},"awayTeam":{"score":1},`+
		`"plays":[{"typeDescKey":"stoppage","periodDescriptor":{"number":3,"periodType":"REG"},"timeRemaining":"00:00"}]}`)
	serveCSV(t, xgCSV)
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)

	result := gp.ProcessGameUpdate(payload)

	if got := n.pushed(t)["gameState"]; got != "Final" {
		t.Errorf("expected Final, got %q", got)
	}
	if result.ShouldReschedule {
		t.Error("a finished game must not be rescheduled")
	}
}

// A CSV parse error on the final cycle must keep retrying: the retry is what
// delivers the final push.
func TestProcessGameUpdate_CSVParseErrorOnFinalGameStillRetries(t *testing.T) {
	servePBP(t, fixtureOT)
	serveCSV(t, "id,homeTeamExpectedGoals\n1,Tkachuk 6'2\" wrister\n")
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)

	result := gp.ProcessGameUpdate(payload)

	n.assertNoPush(t)
	if !result.RetryAfterDataError || !result.ShouldReschedule {
		t.Errorf("expected a short retry so the final push is delivered, got %+v", result)
	}
}

// The exact poll that went wrong: the OT goal and its period-end are in, game-end
// is not yet, and the clock still reads 03:27. gameState is already OFF.
func TestProcessGameUpdate_OvertimePeriodEndBeforeGameEndReportsFinal(t *testing.T) {
	cut := strings.Index(fixtureOT, `,{"typeDescKey":"game-end"`)
	if cut < 0 {
		t.Fatal("fixture no longer ends in game-end")
	}
	servePBP(t, fixtureOT[:cut]+"]}")
	serveCSV(t, xgCSV)
	n := &recordingNotifier{keys: allPushKeys}
	gp, payload := processWith(n)

	result := gp.ProcessGameUpdate(payload)
	data := n.pushed(t)

	if lastPlayType(result.Response) != "period-end" {
		t.Fatalf("expected the poll to end at period-end, got %q", lastPlayType(result.Response))
	}
	if data["gameState"] != "Final" {
		t.Errorf("expected Final, got %q (a clock beside a final score is the bug)", data["gameState"])
	}
	if result.ShouldReschedule {
		t.Error("expected no reschedule; the old code waited 20 minutes here")
	}
}
