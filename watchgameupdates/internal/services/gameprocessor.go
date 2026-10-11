package services

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
)

// ParseErrorRetryInterval is how long to wait before retrying a game check after
// an upstream failure (a malformed MoneyPuck CSV, or the play-by-play fetch
// failing). The failure is usually transient, so we retry the same scenario
// shortly rather than notifying with missing data.
const ParseErrorRetryInterval = 1 * time.Minute

// nonCSVKeys are notifier-required keys the processor fills from the play-by-play
// document or the scheduler's Game payload. Asking MoneyPuck for them is noise:
// gameState, lastPlayType and the tricodes are not CSV columns (each miss logs
// the full column list), and homeTeamGoals/awayTeamGoals exist there but are
// deliberately superseded because MoneyPuck omits the shootout decider.
var nonCSVKeys = map[string]struct{}{
	"gameState": {}, "lastPlayType": {},
	"homeTeamGoals": {}, "awayTeamGoals": {},
	"homeTeamAbbrev": {}, "awayTeamAbbrev": {},
}

const (
	// gameTypePlayoffs is the NHL's gameType for the playoffs (1 preseason, 2 regular
	// season). Only the playoffs take a full intermission before overtime.
	gameTypePlayoffs = 3
	// regulationPeriods is the number of periods before overtime.
	regulationPeriods = 3
)

// GameProcessor contains the shared game-check logic used by both the HTTP handler
// and the asynq worker handler. It is stateless and safe for concurrent use.
type GameProcessor struct {
	Fetcher             GameDataFetcher
	NotificationService *notification.Service
}

// ProcessResult holds the outcome of processing a game update.
type ProcessResult struct {
	ShouldReschedule bool
	// Response is the play-by-play document this result was derived from. Nil when
	// the fetch failed (RetryAfterDataError is then set).
	Response *models.PlayByPlayResponse
	// RetryAfterDataError is set when an upstream failed (the play-by-play fetch,
	// or the MoneyPuck CSV could not be parsed). No notification was sent this
	// cycle; the caller should reschedule a short retry (ParseErrorRetryInterval)
	// rather than the normal interval, and must not consult Response when it is nil.
	RetryAfterDataError bool
}

// NextCheckInterval is how long to wait before the next check. After an upstream
// failure nothing was sent and Response may be nil, so retry soon without deriving
// anything from it; otherwise the document decides.
func (r ProcessResult) NextCheckInterval(cfg *config.Config) time.Duration {
	if r.RetryAfterDataError {
		return ParseErrorRetryInterval
	}
	return RescheduleInterval(r.Response, cfg)
}

// ShouldSkipExecution returns true if the current time is past the execution end window.
func ShouldSkipExecution(payload models.Payload) (bool, error) {
	if payload.ExecutionEnd != nil {
		executionEnd, err := time.Parse(time.RFC3339, *payload.ExecutionEnd)
		if err != nil {
			return true, fmt.Errorf("invalid execution_end format: %w", err)
		}
		if time.Now().After(executionEnd) {
			log.Printf("Current time is after execution end (%s). Skipping execution.", executionEnd.Format(time.RFC3339))
			return true, nil
		}
	} else {
		log.Println("Max execution time not set, proceeding without time check.")
	}
	return false, nil
}

// ProcessGameUpdate runs the core game-check logic: fetch the play-by-play
// document, optionally fetch xG and send notifications, and determine if
// rescheduling is needed. The document is the single source of truth for state,
// clock and score; MoneyPuck contributes expected goals only.
func (gp *GameProcessor) ProcessGameUpdate(payload models.Payload) ProcessResult {
	recomputeTypes := map[string]struct{}{
		"blocked-shot": {},
		"missed-shot":  {},
		"shot-on-goal": {},
		"goal":         {},
		"game-end":     {},
		"period-end":   {},
	}

	resp, err := FetchPlayByPlay(payload.Game.ID, payload.DataSource)
	if err != nil {
		// Nothing to report from: a zeroed push would read as 0-0 "Pregame" on iOS.
		// The self-enqueued retry is the retry; the caller returns 200 so Cloud
		// Tasks does not also retry.
		log.Printf("ERROR: %v; skipping notification and retrying in %s", err, ParseErrorRetryInterval)
		return ProcessResult{
			ShouldReschedule:    ShouldReschedule(payload, nil),
			RetryAfterDataError: true,
		}
	}

	// A finished game always gets a final push, even if gameState flipped to OFF
	// before the last play was a recompute type: this is the last cycle.
	_, recompute := recomputeTypes[lastPlayType(resp)]
	if recompute || IsFinal(resp) {
		log.Printf("Processing play type '%s' for game %s - fetching MoneyPuck data", lastPlayType(resp), payload.Game.ID)

		gameData, err := gp.Fetcher.FetchAndParseGameData(payload.Game.ID, csvKeys(gp.NotificationService.GetAllRequiredDataKeys()), payload.DataSource)

		// A CSV parse error means MoneyPuck served a malformed (usually transient)
		// file. Notifying now would push a zeroed content-state (0-0, empty
		// gameState → "Pregame" on iOS). Instead, send nothing and retry shortly so
		// the next fetch can pick up the corrected file. Reschedule even for a final
		// game: the retry is what delivers the final push.
		if errors.Is(err, ErrCSVParse) {
			log.Printf("WARNING: MoneyPuck CSV parse error for game %s; skipping notification and retrying in %s: %v",
				payload.Game.ID, ParseErrorRetryInterval, err)
			return ProcessResult{
				ShouldReschedule:    true,
				Response:            resp,
				RetryAfterDataError: true,
			}
		}

		if err != nil {
			// MoneyPuck is down but the NHL document is fine: it carries everything
			// the card needs except xG, so push with xG absent (0) rather than a
			// 0-0 "Pregame" card.
			log.Printf("ERROR: Failed to fetch and parse MoneyPuck data for game %s: %v", payload.Game.ID, err)
		}
		if gameData == nil {
			gameData = make(map[string]string)
		}

		gameData["homeTeamGoals"] = strconv.Itoa(resp.HomeTeam.Score)
		gameData["awayTeamGoals"] = strconv.Itoa(resp.AwayTeam.Score)
		gameData["gameState"] = FormatGameState(resp)
		gameData["lastPlayType"] = lastPlayType(resp)

		// An empty gameState is "Pregame" on iOS, which is worse than no update.
		if gameData["gameState"] == "" {
			log.Printf("WARNING: empty gameState for game %s (state=%q); skipping notification", payload.Game.ID, resp.GameState)
		} else {
			gp.NotificationService.SendGameEventNotifications(payload.Game, gameData)
		}
	}

	shouldReschedule := ShouldReschedule(payload, resp)
	log.Printf("Last play type: %s, Should reschedule: %v\n", lastPlayType(resp), shouldReschedule)

	return ProcessResult{
		ShouldReschedule: shouldReschedule,
		Response:         resp,
	}
}

// csvKeys drops the keys the processor supplies itself, leaving only what
// MoneyPuck is the sole source for (expected goals).
func csvKeys(required []string) []string {
	keys := make([]string, 0, len(required))
	for _, k := range required {
		if _, skip := nonCSVKeys[k]; !skip {
			keys = append(keys, k)
		}
	}
	return keys
}

// lastPlayType returns "" for a nil response or one with no plays (pre-game, or
// an update that arrived before the first event); "" matches no recompute type.
func lastPlayType(resp *models.PlayByPlayResponse) string {
	if resp == nil || len(resp.Plays) == 0 {
		return ""
	}
	return resp.Plays[len(resp.Plays)-1].TypeDescKey
}

// IsFinal reports whether the game is over. Nil-safe: a nil response means the
// fetch failed, which is not evidence the game ended. The game-end check backs up
// gameState in case it lags the final horn.
func IsFinal(resp *models.PlayByPlayResponse) bool {
	if resp == nil {
		return false
	}
	// OVER is the NHL's "game ended, not yet official" state; the iOS client
	// treats it as final too (NHLGame.isFinal).
	switch resp.GameState {
	case "OVER", "OFF", "FINAL":
		return true
	}
	return lastPlayType(resp) == "game-end"
}

// ShouldReschedule reports whether to poll again: not past the execution window
// and the game is not over.
func ShouldReschedule(payload models.Payload, resp *models.PlayByPlayResponse) bool {
	if payload.ExecutionEnd != nil {
		executionEnd, err := time.Parse(time.RFC3339, *payload.ExecutionEnd)
		if err != nil {
			log.Printf("Error parsing executionEnd: %v", err)
		} else if time.Now().After(executionEnd) {
			log.Printf("Max execution time (%s) is set and has been reached. Do not reschedule.", executionEnd.Format(time.RFC3339))
			return false
		}
	} else {
		log.Println("Max execution time not set, proceeding without time check.")
	}

	return !IsFinal(resp)
}

// RescheduleInterval returns the next-check interval. Intermissions use the long
// interval except from regulation's end onward outside the playoffs (regular
// season and preseason), where overtime follows in about a minute. The playoffs
// keep the long interval there: their breaks are full intermissions. An unknown or
// missing gameType fails short, so a dropped field costs polls, not a late final push.
func RescheduleInterval(resp *models.PlayByPlayResponse, cfg *config.Config) time.Duration {
	short := time.Duration(cfg.MessageIntervalSeconds) * time.Second
	if resp == nil || !resp.Clock.InIntermission {
		return short
	}
	if resp.PeriodDescriptor.Number >= regulationPeriods && resp.GameType != gameTypePlayoffs {
		return short
	}
	return time.Duration(cfg.PeriodEndIntervalSeconds) * time.Second
}

// FormatGameState returns the display string for the game's current state, derived
// from the document's top-level fields rather than the last play: "Final",
// "Shootout", "End of 2nd" during an intermission, "X:XX left, OT" in overtime,
// or "X:XX left, Nth period". It returns "" when there is nothing to show (a
// game that has not started, or no clock).
func FormatGameState(resp *models.PlayByPlayResponse) string {
	if resp == nil {
		return ""
	}
	if IsFinal(resp) {
		return "Final"
	}
	// Not started: the API still reports period 1 and a full 20:00 clock, so
	// falling through would render "20:00 left, 1st period" before puck drop.
	if resp.GameState == "FUT" || resp.GameState == "PRE" {
		return ""
	}
	pd := resp.PeriodDescriptor
	if pd.PeriodType == "SO" {
		return "Shootout"
	}
	if resp.Clock.InIntermission {
		return "End of " + periodSuffix(pd)
	}
	if resp.Clock.TimeRemaining == "" {
		return ""
	}
	if pd.PeriodType == "OT" {
		return fmt.Sprintf("%s left, OT", resp.Clock.TimeRemaining)
	}
	return fmt.Sprintf("%s left, %s period", resp.Clock.TimeRemaining, periodSuffix(pd))
}

// periodSuffix is "1st", "2nd", "3rd", "4th"... for regulation and "OT" for every
// overtime period (a playoff double overtime also reads "OT").
func periodSuffix(pd models.PeriodDescriptor) string {
	if pd.PeriodType == "OT" {
		return "OT"
	}
	switch pd.Number {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	}
	return fmt.Sprintf("%dth", pd.Number)
}
