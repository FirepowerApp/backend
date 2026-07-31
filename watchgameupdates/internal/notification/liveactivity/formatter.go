package liveactivity

// formatter converts a NotificationRequest into the full dispatch envelope.
//
// FormatMessage output shape (parsed by SendNotification):
//
//   {
//     "channels": ["<base64-channel-id-1>", "<base64-channel-id-2>"],
//     "payload": {
//       "aps": {
//         "timestamp":  1234567890,
//         "event":      "update",         // always — even on game-end, see below
//         "stale-date": 1234568890,       // now + 90s, always — no special-casing
//         "content-state": {
//           "sport":       "nhl",
//           "homeTeam":    "BOS",
//           "awayTeam":    "NYR",
//           "homeScore":   2,
//           "awayScore":   1,
//           "homeXG":      2.4,
//           "awayXG":      1.8,
//           "gameState":   "14:32 left, 2nd period",
//           "eventType":   "goal",       // "goal"|"penalty"|"period_end"|"" (empty = no event)
//           "eventDetail": "",           // scorer name when pipeline lands; empty until then
//           "eventTeam":   "BOS"         // tricode of team that caused the event
//         }
//       }
//     }
//   }

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	. "watchgameupdates/internal/notification"
)

// staleDateOffset applies to every push, including the final one. A game-end
// push is not special at this layer: it is an ordinary content-state update
// whose gameState happens to read "Final", exactly like any other moment's
// gameState reads "14:32 left, 2nd period". The client alone decides what a
// Final gameState means for the activity's lifecycle (see
// LiveActivityManager.endIfFinal) — this backend never sends event:"end",
// since a push with event:"end" is applied by the OS directly with no app
// code in the loop, foreclosing the client's ability to decide anything.
const staleDateOffset = 90 * time.Second

// dispatchEnvelope is what FormatMessage returns and SendNotification parses.
type dispatchEnvelope struct {
	Channels []string        `json:"channels"`
	Payload  json.RawMessage `json:"payload"`
}

type contentState struct {
	Sport       string  `json:"sport"`
	HomeTeam    string  `json:"homeTeam"`
	AwayTeam    string  `json:"awayTeam"`
	HomeScore   int     `json:"homeScore"`
	AwayScore   int     `json:"awayScore"`
	HomeXG      float64 `json:"homeXG"`
	AwayXG      float64 `json:"awayXG"`
	GameState   string  `json:"gameState"`
	EventType   string  `json:"eventType"`   // "goal"|"penalty"|"period_end"|""
	EventDetail string  `json:"eventDetail"` // scorer name when available; "" until then
	EventTeam   string  `json:"eventTeam"`   // scoring/penalised team tricode; "" if unknown
}

type apsEnvelope struct {
	Timestamp    int64        `json:"timestamp"`
	Event        string       `json:"event"`
	StaleDate    *int64       `json:"stale-date,omitempty"`
	ContentState contentState `json:"content-state"`
}

type apnsPayload struct {
	APS apsEnvelope `json:"aps"`
}

// BuildDispatchMessage produces the JSON string for FormatMessage.
// useDevChannels selects sandbox APNs channel IDs (debug builds) vs production.
func BuildDispatchMessage(req NotificationRequest, useDevChannels bool) (string, error) {
	cs, err := buildContentState(req)
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	ts := now + int64(staleDateOffset.Seconds())

	aps := apsEnvelope{
		Timestamp:    now,
		Event:        "update",
		StaleDate:    &ts,
		ContentState: cs,
	}

	payloadBytes, err := json.Marshal(apnsPayload{APS: aps})
	if err != nil {
		return "", fmt.Errorf("marshal APNs payload: %w", err)
	}

	homeAbbrev := strings.ToUpper(req.Data["homeTeamAbbrev"])
	awayAbbrev := strings.ToUpper(req.Data["awayTeamAbbrev"])

	channels := channelsForTeams(homeAbbrev, awayAbbrev, useDevChannels)
	if len(channels) == 0 {
		return "", fmt.Errorf("no channel IDs registered for %s or %s", homeAbbrev, awayAbbrev)
	}

	env := dispatchEnvelope{
		Channels: channels,
		Payload:  json.RawMessage(payloadBytes),
	}

	b, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("marshal dispatch envelope: %w", err)
	}
	return string(b), nil
}

func buildContentState(req NotificationRequest) (contentState, error) {
	homeScore := parseIntSafe(req.Data["homeTeamGoals"])
	awayScore := parseIntSafe(req.Data["awayTeamGoals"])

	eventType, eventTeam := classifyEvent(req.Data["lastPlayType"], req.Data)

	return contentState{
		Sport:       "nhl",
		HomeTeam:    strings.ToUpper(req.Data["homeTeamAbbrev"]),
		AwayTeam:    strings.ToUpper(req.Data["awayTeamAbbrev"]),
		HomeScore:   homeScore,
		AwayScore:   awayScore,
		HomeXG:      safeXG(req.Data["homeTeamExpectedGoals"]),
		AwayXG:      safeXG(req.Data["awayTeamExpectedGoals"]),
		GameState:   req.Data["gameState"],
		EventType:   eventType,
		EventDetail: "", // scorer name: empty until the play-by-play pipeline lands (TODO)
		EventTeam:   eventTeam,
	}, nil
}

// classifyEvent returns (eventType, eventTeam) for the current play.
// eventDetail (scorer name) is a separate TODO — see planning/todos.md.
func classifyEvent(playType string, data map[string]string) (eventType, eventTeam string) {
	switch playType {
	case "goal":
		return "goal", strings.ToUpper(data["eventTeamAbbrev"])
	case "penalty":
		return "penalty", strings.ToUpper(data["eventTeamAbbrev"])
	case "period-end":
		return "period_end", ""
	case "game-end":
		return "period_end", ""
	default:
		return "", ""
	}
}

// safeXG parses an xG string and guards against NaN/Inf.
// The value passes through exactly as sourced — no rounding. Display
// formatting is the iOS client's responsibility (see CLAUDE.md).
func safeXG(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		log.Printf("WARN: invalid xG value %q, defaulting to 0", s)
		return 0
	}
	return v
}

func parseIntSafe(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// channelsForTeams returns the APNs broadcast channel IDs for the two teams.
// Teams whose channel ID has not been populated in channels.go are skipped.
func channelsForTeams(homeAbbrev, awayAbbrev string, useDevChannels bool) []string {
	var channels []string
	if id, ok := channelForTeam(homeAbbrev, useDevChannels); ok {
		channels = append(channels, id)
	} else {
		log.Printf("WARN: no channel ID for home team %s, skipping", homeAbbrev)
	}
	if awayAbbrev != "" && awayAbbrev != homeAbbrev {
		if id, ok := channelForTeam(awayAbbrev, useDevChannels); ok {
			channels = append(channels, id)
		} else {
			log.Printf("WARN: no channel ID for away team %s, skipping", awayAbbrev)
		}
	}
	return channels
}
