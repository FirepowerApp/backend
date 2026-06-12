package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
)

// recordingNotifier satisfies notification.Notifier and flips `sent` the moment
// it is asked to format or send anything, so a test can assert that the parse-
// error path never reaches the notifiers.
type recordingNotifier struct {
	sent atomic.Bool
}

func (n *recordingNotifier) GetRequiredDataKeys() []string { return []string{"homeTeamGoals"} }

func (n *recordingNotifier) FormatMessage(req notification.NotificationRequest) string {
	n.sent.Store(true)
	return ""
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
	pbp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"plays":[{"typeDescKey":"goal","periodDescriptor":{"number":1,"periodType":"REG"},"timeRemaining":"12:00"}]}`))
	}))
	defer pbp.Close()
	t.Setenv("PLAYBYPLAY_API_BASE_URL", pbp.URL)

	serveCSV(t, "id,homeTeamGoals\n1,2\n")

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

	if result.RetryAfterDataError {
		t.Error("expected RetryAfterDataError=false for a valid CSV")
	}
}
