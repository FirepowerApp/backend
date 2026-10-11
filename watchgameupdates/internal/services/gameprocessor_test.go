package services

import (
	"testing"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
)

// pbp builds a play-by-play response for table tests. Pass "" for lastPlay to get
// an empty plays slice.
func pbp(gameType int, state string, num int, periodType string, inIntermission bool, clock, lastPlay string) *models.PlayByPlayResponse {
	r := &models.PlayByPlayResponse{
		GameType:         gameType,
		GameState:        state,
		PeriodDescriptor: models.PeriodDescriptor{Number: num, PeriodType: periodType},
		Clock:            models.Clock{TimeRemaining: clock, InIntermission: inIntermission},
	}
	if lastPlay != "" {
		r.Plays = []models.Play{{TypeDescKey: lastPlay}}
	}
	return r
}

func TestFormatGameState(t *testing.T) {
	testCases := []struct {
		name     string
		resp     *models.PlayByPlayResponse
		expected string
	}{
		// Live: these strings are a wire contract pinned since #32; do not change.
		{"FirstPeriod", pbp(2, "LIVE", 1, "REG", false, "15:32", "shot-on-goal"), "15:32 left, 1st period"},
		{"SecondPeriod", pbp(2, "LIVE", 2, "REG", false, "06:56", "goal"), "06:56 left, 2nd period"},
		{"ThirdPeriod", pbp(2, "LIVE", 3, "REG", false, "01:45", "missed-shot"), "01:45 left, 3rd period"},
		{"FourthRegPeriodFallbackSuffix", pbp(2, "LIVE", 4, "REG", false, "10:00", "blocked-shot"), "10:00 left, 4th period"},
		{"Overtime", pbp(2, "LIVE", 4, "OT", false, "03:22", "shot-on-goal"), "03:22 left, OT"},
		{"PlayoffDoubleOvertimeStillReadsOT", pbp(3, "LIVE", 5, "OT", false, "04:12", "shot-on-goal"), "04:12 left, OT"},
		{"ShootoutInProgress", pbp(2, "LIVE", 5, "SO", false, "00:00", "shot-on-goal"), "Shootout"},

		// Final: the reported bug. The OT period-end still carries the clock from the
		// winning goal, but the document says the game is over.
		{"OTPeriodEndOnAGoalIsFinalNotARunningClock", pbp(2, "OFF", 4, "OT", false, "03:27", "period-end"), "Final"},
		{"GameEnd", pbp(2, "LIVE", 3, "REG", false, "00:00", "game-end"), "Final"},
		{"GameStateFinal", pbp(2, "FINAL", 3, "REG", false, "00:00", "shot-on-goal"), "Final"},
		// OVER: the game has ended but is not yet official. The last play can still be
		// the period-end carrying the winning goal's clock.
		{"GameStateOverWithPeriodEndAsLastPlay", pbp(2, "OVER", 4, "OT", false, "03:27", "period-end"), "Final"},
		{"DecidedShootoutIsFinalNotShootout", pbp(2, "OFF", 5, "SO", false, "00:00", "game-end"), "Final"},

		// Intermissions: the one new family of wire values.
		{"EndOfFirst", pbp(2, "LIVE", 1, "REG", true, "17:42", "period-end"), "End of 1st"},
		{"EndOfSecond", pbp(2, "LIVE", 2, "REG", true, "17:42", "period-end"), "End of 2nd"},
		{"EndOfThird", pbp(2, "LIVE", 3, "REG", true, "01:00", "period-end"), "End of 3rd"},
		{"EndOfOT", pbp(3, "LIVE", 4, "OT", true, "17:00", "period-end"), "End of OT"},

		// Nothing to show: an empty string is "Pregame" on iOS, so callers skip the push.
		{"FutureGameHasFullClockButNotStarted", pbp(2, "FUT", 1, "REG", false, "20:00", ""), ""},
		{"PreGame", pbp(2, "PRE", 1, "REG", false, "20:00", "period-start"), ""},
		{"LiveWithNoClock", pbp(2, "LIVE", 1, "REG", false, "", "period-start"), ""},
		{"NilResponse", nil, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatGameState(tc.resp); got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestRescheduleInterval(t *testing.T) {
	cfg := &config.Config{MessageIntervalSeconds: 60, PeriodEndIntervalSeconds: 1200}
	short, long := 60*time.Second, 1200*time.Second

	testCases := []struct {
		name     string
		resp     *models.PlayByPlayResponse
		expected time.Duration
	}{
		{"NilResponse", nil, short},
		{"NotInIntermission", pbp(2, "LIVE", 2, "REG", false, "10:00", "shot-on-goal"), short},
		// The reported bug: an OT period-end on a goal is not an intermission.
		{"OTPeriodEndOnAGoalIsNotAnIntermission", pbp(2, "OFF", 4, "OT", false, "03:27", "period-end"), short},

		{"RegularSeason_P1", pbp(2, "LIVE", 1, "REG", true, "17:42", "period-end"), long},
		{"RegularSeason_P2", pbp(2, "LIVE", 2, "REG", true, "17:42", "period-end"), long},
		// Overtime follows in about a minute.
		{"RegularSeason_P3", pbp(2, "LIVE", 3, "REG", true, "01:00", "period-end"), short},
		{"RegularSeason_P4", pbp(2, "LIVE", 4, "OT", true, "01:00", "period-end"), short},

		// Preseason (gameType 1) shares the regular season's overtime rules (the old
		// maxPeriods check was present for it too), and a response missing gameType
		// must fail short, never sleep through an overtime.
		{"Preseason_P3", pbp(1, "LIVE", 3, "REG", true, "01:00", "period-end"), short},
		{"Preseason_P1", pbp(1, "LIVE", 1, "REG", true, "17:42", "period-end"), long},
		{"MissingGameType_P3", pbp(0, "LIVE", 3, "REG", true, "01:00", "period-end"), short},

		// Playoff breaks are full intermissions. Asserted before this change at
		// Playoffs_Period3 / Playoffs_OTPeriod4; sending 60s here would push ~18
		// identical Live Activity updates per break.
		{"Playoffs_P1", pbp(3, "LIVE", 1, "REG", true, "17:42", "period-end"), long},
		{"Playoffs_P2", pbp(3, "LIVE", 2, "REG", true, "17:42", "period-end"), long},
		{"Playoffs_P3", pbp(3, "LIVE", 3, "REG", true, "17:42", "period-end"), long},
		{"Playoffs_OT1", pbp(3, "LIVE", 4, "OT", true, "17:42", "period-end"), long},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RescheduleInterval(tc.resp, cfg); got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestIsFinalAndLastPlayType(t *testing.T) {
	if IsFinal(nil) {
		t.Error("a nil response (failed fetch) must not read as a finished game")
	}
	if got := lastPlayType(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
	if got := lastPlayType(pbp(2, "LIVE", 1, "REG", false, "20:00", "")); got != "" {
		t.Errorf("expected empty for no plays, got %q", got)
	}
	if IsFinal(pbp(2, "LIVE", 1, "REG", false, "20:00", "")) {
		t.Error("a live game with no plays yet is not final")
	}
}

func TestCSVKeysKeepsOnlyWhatMoneyPuckAloneProvides(t *testing.T) {
	required := []string{
		"homeTeamGoals", "awayTeamGoals", "homeTeamExpectedGoals", "awayTeamExpectedGoals",
		"gameState", "homeTeamAbbrev", "awayTeamAbbrev", "lastPlayType",
	}
	got := csvKeys(required)
	if len(got) != 2 || got[0] != "homeTeamExpectedGoals" || got[1] != "awayTeamExpectedGoals" {
		t.Errorf("expected only the two xG keys, got %v", got)
	}
}

func TestShouldSkipExecution(t *testing.T) {
	t.Run("NilExecutionEnd_ShouldNotSkip", func(t *testing.T) {
		payload := models.Payload{
			Game: models.Game{ID: "2024030411"},
		}

		skip, err := ShouldSkipExecution(payload)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if skip {
			t.Error("Expected skip=false for nil ExecutionEnd")
		}
	})

	t.Run("FutureExecutionEnd_ShouldNotSkip", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &future,
		}

		skip, err := ShouldSkipExecution(payload)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if skip {
			t.Error("Expected skip=false for future ExecutionEnd")
		}
	})

	t.Run("PastExecutionEnd_ShouldSkip", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &past,
		}

		skip, err := ShouldSkipExecution(payload)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !skip {
			t.Error("Expected skip=true for past ExecutionEnd")
		}
	})

	t.Run("InvalidExecutionEnd_ReturnsError", func(t *testing.T) {
		invalid := "not-a-date"
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &invalid,
		}

		skip, err := ShouldSkipExecution(payload)
		if err == nil {
			t.Error("Expected error for invalid ExecutionEnd format")
		}
		if !skip {
			t.Error("Expected skip=true when ExecutionEnd is invalid")
		}
	})
}

func TestShouldReschedule(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	game := models.Game{ID: "2024030411"}

	testCases := []struct {
		name     string
		end      *string
		resp     *models.PlayByPlayResponse
		expected bool
	}{
		{"NonFinal", &future, pbp(2, "LIVE", 2, "REG", false, "10:00", "shot-on-goal"), true},
		{"GameEnd", &future, pbp(2, "LIVE", 3, "REG", false, "00:00", "game-end"), false},
		// The OT bug fixture: the last play is a period-end, but the game is over.
		{"GameStateOffWithPeriodEndAsLastPlay", &future, pbp(2, "OFF", 4, "OT", false, "03:27", "period-end"), false},
		{"GameStateOverWithPeriodEndAsLastPlay", &future, pbp(2, "OVER", 4, "OT", false, "03:27", "period-end"), false},
		// A failed fetch is not evidence the game ended: keep polling.
		{"NilResponseKeepsPolling", &future, nil, true},
		{"NilExecutionEnd", nil, pbp(2, "LIVE", 2, "REG", false, "10:00", "shot-on-goal"), true},
		{"PastExecutionEnd", &past, pbp(2, "LIVE", 2, "REG", false, "10:00", "shot-on-goal"), false},
		{"PastExecutionEndOnFailedFetch", &past, nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := models.Payload{Game: game, ExecutionEnd: tc.end}
			if got := ShouldReschedule(payload, tc.resp); got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestNextCheckInterval(t *testing.T) {
	cfg := &config.Config{MessageIntervalSeconds: 60, PeriodEndIntervalSeconds: 1200}

	// An upstream failure leaves Response nil: retry in ParseErrorRetryInterval and
	// do not touch the response. This ordering is what keeps a nil Response safe.
	failed := ProcessResult{RetryAfterDataError: true}
	if got := failed.NextCheckInterval(cfg); got != ParseErrorRetryInterval {
		t.Errorf("expected %v after an upstream failure, got %v", ParseErrorRetryInterval, got)
	}

	// A MoneyPuck parse error carries a real Response but still retries quickly,
	// even mid-way through an intermission that would otherwise wait 20 minutes.
	parseErr := ProcessResult{RetryAfterDataError: true, Response: pbp(2, "LIVE", 1, "REG", true, "17:42", "period-end")}
	if got := parseErr.NextCheckInterval(cfg); got != ParseErrorRetryInterval {
		t.Errorf("expected %v on a parse error, got %v", ParseErrorRetryInterval, got)
	}

	normal := ProcessResult{Response: pbp(2, "LIVE", 1, "REG", true, "17:42", "period-end")}
	if got := normal.NextCheckInterval(cfg); got != 1200*time.Second {
		t.Errorf("expected the document to decide (1200s), got %v", got)
	}
}
