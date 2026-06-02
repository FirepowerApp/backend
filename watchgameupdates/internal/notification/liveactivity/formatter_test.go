package liveactivity

import (
	"encoding/json"
	"math"
	"testing"

	. "watchgameupdates/internal/notification"
)

// withChannels temporarily sets channel IDs in the prod or dev map for the duration
// of fn, then restores the originals. Allows tests to exercise channel lookup without
// mutating the real maps permanently.
func withChannels(t *testing.T, ids map[string]string, useDevChannels bool, fn func()) {
	t.Helper()
	m := prodChannels
	if useDevChannels {
		m = debugChannels
	}
	orig := map[string]string{}
	for k, v := range m {
		orig[k] = v
	}
	for k, v := range ids {
		m[k] = v
	}
	t.Cleanup(func() {
		for k, v := range orig {
			m[k] = v
		}
	})
	fn()
}

func baseReq() NotificationRequest {
	return NotificationRequest{
		Team1ID: "Bruins",
		Team2ID: "Rangers",
		Data: map[string]string{
			"homeTeamGoals":         "2",
			"awayTeamGoals":         "1",
			"homeTeamExpectedGoals": "2.4",
			"awayTeamExpectedGoals": "1.8",
			"homeTeamAbbrev":        "BOS",
			"awayTeamAbbrev":        "NYR",
			"gameState":             "14:32 left, 2nd period",
			"lastPlayType":          "goal",
		},
	}
}

func TestBuildDispatchMessage_HappyPath(t *testing.T) {
	withChannels(t, map[string]string{"BOS": "chan-BOS", "NYR": "chan-NYR"}, false, func() {
		msg, err := BuildDispatchMessage(baseReq(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var env dispatchEnvelope
		if err := json.Unmarshal([]byte(msg), &env); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}

		if len(env.Channels) != 2 {
			t.Errorf("want 2 channels, got %d", len(env.Channels))
		}
		if env.Channels[0] != "chan-BOS" {
			t.Errorf("want chan-BOS, got %s", env.Channels[0])
		}
		if env.Channels[1] != "chan-NYR" {
			t.Errorf("want chan-NYR, got %s", env.Channels[1])
		}

		var payload apnsPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			t.Fatalf("payload not valid JSON: %v", err)
		}

		cs := payload.APS.ContentState
		if cs.HomeScore != 2 {
			t.Errorf("want HomeScore=2, got %d", cs.HomeScore)
		}
		if cs.AwayScore != 1 {
			t.Errorf("want AwayScore=1, got %d", cs.AwayScore)
		}
		if cs.HomeXG != 2.4 {
			t.Errorf("want HomeXG=2.4, got %f", cs.HomeXG)
		}
		if cs.Sport != "nhl" {
			t.Errorf("want sport=nhl, got %s", cs.Sport)
		}
		if payload.APS.Event != "update" {
			t.Errorf("want event=update, got %s", payload.APS.Event)
		}
		if payload.APS.StaleDate == nil {
			t.Error("want non-nil stale-date for in-progress game")
		}
	})
}

func TestBuildDispatchMessage_GameEnded(t *testing.T) {
	withChannels(t, map[string]string{"BOS": "chan-BOS", "NYR": "chan-NYR"}, false, func() {
		req := baseReq()
		req.Data["gameState"] = "Final"
		req.Data["lastPlayType"] = "game-end"

		msg, err := BuildDispatchMessage(req, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var env dispatchEnvelope
		json.Unmarshal([]byte(msg), &env)

		var payload apnsPayload
		json.Unmarshal(env.Payload, &payload)

		if payload.APS.Event != "end" {
			t.Errorf("want event=end for game-end, got %s", payload.APS.Event)
		}
		if payload.APS.DismissalDate == nil {
			t.Error("want non-nil dismissal-date for game-end")
		}
		if payload.APS.StaleDate != nil {
			t.Error("want nil stale-date for game-end")
		}
	})
}

func TestBuildDispatchMessage_NilEventString(t *testing.T) {
	withChannels(t, map[string]string{"BOS": "chan-BOS", "NYR": "chan-NYR"}, false, func() {
		req := baseReq()
		delete(req.Data, "lastPlayType")

		msg, err := BuildDispatchMessage(req, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var env dispatchEnvelope
		json.Unmarshal([]byte(msg), &env)

		var payload apnsPayload
		json.Unmarshal(env.Payload, &payload)

		if payload.APS.ContentState.LastEvent != "" {
			t.Errorf("want empty lastEvent for missing playType, got %q", payload.APS.ContentState.LastEvent)
		}
	})
}

func TestBuildDispatchMessage_NoChannelsRegistered(t *testing.T) {
	// Both teams have empty channel IDs — should error rather than push to nothing.
	_, err := BuildDispatchMessage(baseReq(), false)
	if err == nil {
		t.Fatal("want error when no channel IDs are registered, got nil")
	}
}

func TestBuildDispatchMessage_OneChannelRegistered(t *testing.T) {
	// Only home team has a channel ID — push to that one, skip away.
	withChannels(t, map[string]string{"BOS": "chan-BOS"}, false, func() {
		msg, err := BuildDispatchMessage(baseReq(), false)
		if err != nil {
			t.Fatalf("unexpected error when one channel registered: %v", err)
		}

		var env dispatchEnvelope
		json.Unmarshal([]byte(msg), &env)

		if len(env.Channels) != 1 {
			t.Errorf("want 1 channel, got %d", len(env.Channels))
		}
		if env.Channels[0] != "chan-BOS" {
			t.Errorf("want chan-BOS, got %s", env.Channels[0])
		}
	})
}

func TestBuildDispatchMessage_MissingScores(t *testing.T) {
	withChannels(t, map[string]string{"BOS": "chan-BOS", "NYR": "chan-NYR"}, false, func() {
		req := baseReq()
		delete(req.Data, "homeTeamGoals")
		delete(req.Data, "awayTeamGoals")

		msg, err := BuildDispatchMessage(req, false)
		if err != nil {
			t.Fatalf("missing scores should not error: %v", err)
		}

		var env dispatchEnvelope
		json.Unmarshal([]byte(msg), &env)

		var payload apnsPayload
		json.Unmarshal(env.Payload, &payload)

		cs := payload.APS.ContentState
		if cs.HomeScore != 0 || cs.AwayScore != 0 {
			t.Errorf("missing scores should default to 0, got home=%d away=%d", cs.HomeScore, cs.AwayScore)
		}
	})
}

func TestSafeXG_NaN(t *testing.T) {
	if got := safeXG("NaN"); got != 0 {
		t.Errorf("safeXG(NaN) = %v, want 0", got)
	}
}

func TestSafeXG_Inf(t *testing.T) {
	if got := safeXG("Inf"); got != 0 {
		t.Errorf("safeXG(Inf) = %v, want 0", got)
	}
}

func TestSafeXG_NegInf(t *testing.T) {
	if got := safeXG("-Inf"); got != 0 {
		t.Errorf("safeXG(-Inf) = %v, want 0", got)
	}
}

func TestSafeXG_Empty(t *testing.T) {
	if got := safeXG(""); got != 0 {
		t.Errorf("safeXG(\"\") = %v, want 0", got)
	}
}

func TestSafeXG_ValidFloat(t *testing.T) {
	if got := safeXG("2.456"); got != 2.5 {
		t.Errorf("safeXG(2.456) = %v, want 2.5", got)
	}
}

func TestSafeXG_ActualNaN(t *testing.T) {
	v := math.NaN()
	if !math.IsNaN(v) {
		t.Fatal("sanity: math.NaN() is not NaN")
	}
}

func TestFormatLastEvent(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"goal", "Goal scored"},
		{"shot-on-goal", "Shot on goal"},
		{"blocked-shot", "Shot blocked"},
		{"missed-shot", "Shot missed"},
		{"period-end", "Period ended"},
		{"game-end", "Final"},
		{"", ""},
		{"faceoff", ""},
		{"unknown-type", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := formatLastEvent(tc.input)
			if got != tc.want {
				t.Errorf("formatLastEvent(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestChannelsForTeams_BothRegistered(t *testing.T) {
	withChannels(t, map[string]string{"BOS": "chan-BOS", "NYR": "chan-NYR"}, false, func() {
		ch := channelsForTeams("BOS", "NYR", false)
		if len(ch) != 2 {
			t.Fatalf("want 2 channels, got %d", len(ch))
		}
		if ch[0] != "chan-BOS" || ch[1] != "chan-NYR" {
			t.Errorf("unexpected channels: %v", ch)
		}
	})
}

func TestChannelsForTeams_NoneRegistered(t *testing.T) {
	ch := channelsForTeams("BOS", "NYR", false)
	if len(ch) != 0 {
		t.Errorf("want 0 channels when none registered, got %d", len(ch))
	}
}

func TestChannelsForTeams_SameTeam(t *testing.T) {
	withChannels(t, map[string]string{"BOS": "chan-BOS"}, false, func() {
		ch := channelsForTeams("BOS", "BOS", false)
		if len(ch) != 1 {
			t.Errorf("same-team edge case: want 1 channel, got %d", len(ch))
		}
	})
}
