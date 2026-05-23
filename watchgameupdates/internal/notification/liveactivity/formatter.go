package liveactivity

// formatter converts a NotificationRequest into the full dispatch envelope.
//
// FormatMessage output shape (parsed by SendNotification):
//
//   {
//     "channels": ["nhl-team-BOS", "nhl-team-NYR"],
//     "payload": {
//       "aps": {
//         "timestamp":  1234567890,
//         "event":      "update" | "end",
//         "stale-date": 1234568890,       // update only: now + 90s
//         "dismissal-date": 1234569490,   // end only: now + 10min
//         "content-state": {
//           "sport":     "nhl",
//           "homeTeam":  "BOS",
//           "awayTeam":  "NYR",
//           "homeScore": 2,
//           "awayScore": 1,
//           "homeXG":    2.4,
//           "awayXG":    1.8,
//           "gameState": "14:32 left, 2nd period",
//           "lastEvent": "Goal scored"
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

const (
	staleDateOffset = 90 * time.Second
	dismissalOffset = 10 * time.Minute
)

// dispatchEnvelope is what FormatMessage returns and SendNotification parses.
type dispatchEnvelope struct {
	Channels []string        `json:"channels"`
	Payload  json.RawMessage `json:"payload"`
}

type contentState struct {
	Sport     string  `json:"sport"`
	HomeTeam  string  `json:"homeTeam"`
	AwayTeam  string  `json:"awayTeam"`
	HomeScore int     `json:"homeScore"`
	AwayScore int     `json:"awayScore"`
	HomeXG    float64 `json:"homeXG"`
	AwayXG    float64 `json:"awayXG"`
	GameState string  `json:"gameState"`
	LastEvent string  `json:"lastEvent"`
}

type apsEnvelope struct {
	Timestamp     int64        `json:"timestamp"`
	Event         string       `json:"event"`
	StaleDate     *int64       `json:"stale-date,omitempty"`
	DismissalDate *int64       `json:"dismissal-date,omitempty"`
	ContentState  contentState `json:"content-state"`
}

type apnsPayload struct {
	APS apsEnvelope `json:"aps"`
}

// BuildDispatchMessage produces the JSON string for FormatMessage.
func BuildDispatchMessage(req NotificationRequest) (string, error) {
	cs, err := buildContentState(req)
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	isEnded := strings.EqualFold(req.Data["gameState"], "final") ||
		strings.EqualFold(req.Data["lastPlayType"], "game-end")

	aps := apsEnvelope{
		Timestamp:    now,
		ContentState: cs,
	}
	if isEnded {
		aps.Event = "end"
		ts := now + int64(dismissalOffset.Seconds())
		aps.DismissalDate = &ts
	} else {
		aps.Event = "update"
		ts := now + int64(staleDateOffset.Seconds())
		aps.StaleDate = &ts
	}

	payloadBytes, err := json.Marshal(apnsPayload{APS: aps})
	if err != nil {
		return "", fmt.Errorf("marshal APNs payload: %w", err)
	}

	homeAbbrev := strings.ToUpper(req.Data["homeTeamAbbrev"])
	awayAbbrev := strings.ToUpper(req.Data["awayTeamAbbrev"])

	channels := channelsForTeams(homeAbbrev, awayAbbrev)
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

	return contentState{
		Sport:     "nhl",
		HomeTeam:  strings.ToUpper(req.Data["homeTeamAbbrev"]),
		AwayTeam:  strings.ToUpper(req.Data["awayTeamAbbrev"]),
		HomeScore: homeScore,
		AwayScore: awayScore,
		HomeXG:    safeXG(req.Data["homeTeamExpectedGoals"]),
		AwayXG:    safeXG(req.Data["awayTeamExpectedGoals"]),
		GameState: req.Data["gameState"],
		LastEvent: formatLastEvent(req.Data["lastPlayType"]),
	}, nil
}

// safeXG parses an xG string, returning 0 for empty, unparseable, NaN, or Inf values.
func safeXG(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		log.Printf("WARN: invalid xG value %q, defaulting to 0", s)
		return 0
	}
	return math.Round(v*10) / 10
}

func parseIntSafe(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// formatLastEvent converts a play TypeDescKey to a human-readable string.
// Empty or unrecognized types return "" safely.
func formatLastEvent(playType string) string {
	switch playType {
	case "goal":
		return "Goal scored"
	case "shot-on-goal":
		return "Shot on goal"
	case "blocked-shot":
		return "Shot blocked"
	case "missed-shot":
		return "Shot missed"
	case "period-end":
		return "Period ended"
	case "game-end":
		return "Final"
	default:
		return "" // empty string is safe for the iOS lastEvent optional
	}
}

// channelsForTeams returns the APNs broadcast channel IDs for the two teams.
// Teams whose channel ID has not been populated in channels.go are skipped.
func channelsForTeams(homeAbbrev, awayAbbrev string) []string {
	var channels []string
	if id, ok := channelForTeam(homeAbbrev); ok {
		channels = append(channels, id)
	} else {
		log.Printf("WARN: no channel ID for home team %s, skipping", homeAbbrev)
	}
	if awayAbbrev != "" && awayAbbrev != homeAbbrev {
		if id, ok := channelForTeam(awayAbbrev); ok {
			channels = append(channels, id)
		} else {
			log.Printf("WARN: no channel ID for away team %s, skipping", awayAbbrev)
		}
	}
	return channels
}
