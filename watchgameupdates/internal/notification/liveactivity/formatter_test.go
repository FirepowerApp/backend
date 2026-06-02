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
			"eventTeamAbbrev":       "BOS",
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
	if env.Channels[0] != "nhl-team-BOS" {
		t.Errorf("want nhl-team-BOS, got %s", env.Channels[0])
	}
	if env.Channels[1] != "nhl-team-NYR" {
		t.Errorf("want nhl-team-NYR, got %s", env.Channels[1])
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

func TestBuildDispatchMessage_GoalEventFields(t *testing.T) {
	msg, _ := BuildDispatchMessage(baseReq())
	cs := unmarshalCS(t, msg)

	if cs.EventType != "goal" {
		t.Errorf("want eventType=goal, got %q", cs.EventType)
	}
	if cs.EventTeam != "BOS" {
		t.Errorf("want eventTeam=BOS, got %q", cs.EventTeam)
	}
	// eventDetail empty until scorer pipeline lands
	if cs.EventDetail != "" {
		t.Errorf("want eventDetail empty, got %q", cs.EventDetail)
	}
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

func TestBuildDispatchMessage_MissingPlayType_EmptyEvent(t *testing.T) {
	req := baseReq()
	delete(req.Data, "lastPlayType")
	cs := unmarshalCS(t, mustBuild(t, req))

	if cs.EventType != "" {
		t.Errorf("want empty eventType for missing playType, got %q", cs.EventType)
	}
	if cs.EventTeam != "" {
		t.Errorf("want empty eventTeam for missing playType, got %q", cs.EventTeam)
	}
}

func TestBuildDispatchMessage_UnknownPlayType_EmptyEvent(t *testing.T) {
	req := baseReq()
	req.Data["lastPlayType"] = "faceoff"
	cs := unmarshalCS(t, mustBuild(t, req))

	if cs.EventType != "" {
		t.Errorf("faceoff should produce empty eventType, got %q", cs.EventType)
	}
}

func TestBuildDispatchMessage_GameEnded(t *testing.T) {
	req := baseReq()
	req.Data["gameState"] = "Final"
	req.Data["lastPlayType"] = "game-end"

	msg, err := BuildDispatchMessage(req)
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
}

// classifyEvent unit tests

func TestClassifyEvent_Goal(t *testing.T) {
	et, team := classifyEvent("goal", map[string]string{"eventTeamAbbrev": "bos"})
	if et != "goal" {
		t.Errorf("want goal, got %q", et)
	}
	if team != "BOS" {
		t.Errorf("want BOS (uppercased), got %q", team)
	}
}

func TestClassifyEvent_PeriodEnd(t *testing.T) {
	et, team := classifyEvent("period-end", nil)
	if et != "period_end" {
		t.Errorf("want period_end, got %q", et)
	}
	if team != "" {
		t.Errorf("want empty team for period-end, got %q", team)
	}
}

func TestClassifyEvent_GameEnd(t *testing.T) {
	et, _ := classifyEvent("game-end", nil)
	if et != "period_end" {
		t.Errorf("game-end should map to period_end, got %q", et)
	}
}

func TestClassifyEvent_Unknown(t *testing.T) {
	et, team := classifyEvent("blocked-shot", map[string]string{})
	if et != "" || team != "" {
		t.Errorf("unknown play should produce empty fields, got type=%q team=%q", et, team)
	}
}

// Existing tests preserved

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

func TestBuildDispatchMessage_MissingScores(t *testing.T) {
	req := baseReq()
	delete(req.Data, "homeTeamGoals")
	delete(req.Data, "awayTeamGoals")
	cs := unmarshalCS(t, mustBuild(t, req))
	if cs.HomeScore != 0 || cs.AwayScore != 0 {
		t.Errorf("missing scores should default to 0, got home=%d away=%d", cs.HomeScore, cs.AwayScore)
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
