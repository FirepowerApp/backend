package services

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
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
	LastPlay         models.Play
	MaxPeriods       *int
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

// ProcessGameUpdate runs the core game-check logic: fetch play-by-play data,
// optionally fetch stats and send notifications, and determine if rescheduling is needed.
func (gp *GameProcessor) ProcessGameUpdate(payload models.Payload) ProcessResult {
	recomputeTypes := map[string]struct{}{
		"blocked-shot": {},
		"missed-shot":  {},
		"shot-on-goal": {},
		"goal":         {},
		"game-end":     {},
		"period-end":   {},
	}

	lastPlay, maxPeriods := FetchPlayByPlay(payload.Game.ID)

	if _, ok := recomputeTypes[lastPlay.TypeDescKey]; ok {
		log.Printf("Processing play type '%s' for game %s - fetching MoneyPuck data", lastPlay.TypeDescKey, payload.Game.ID)

		requiredKeys := gp.NotificationService.GetAllRequiredDataKeys()
		gameData, err := gp.Fetcher.FetchAndParseGameData(payload.Game.ID, requiredKeys)
		if err != nil {
			log.Printf("ERROR: Failed to fetch and parse MoneyPuck data for game %s: %v", payload.Game.ID, err)
		}

		// Populate game state (period/time) from play-by-play data.
		if gameData != nil {
			gameData["gameState"] = FormatGameState(lastPlay)
		}

		// Only run shootout adjustment when the fetch succeeded — a partial map
		// could otherwise let the adjustment run on inconsistent data.
		if err == nil && lastPlay.TypeDescKey == "game-end" && gameData != nil {
			homeGoals, homeGOK := gameData["homeTeamGoals"]
			awayGoals, awayGOK := gameData["awayTeamGoals"]
			if homeGOK && awayGOK && homeGoals == awayGoals {
				if shootoutErr := AdjustScoreForShootout(gameData); shootoutErr != nil {
					log.Printf("Failed to adjust score for shootout: %v", shootoutErr)
				}
			}
		}

		gp.NotificationService.SendGameEventNotifications(payload.Game, gameData)
	}

	shouldReschedule := ShouldReschedule(payload, lastPlay)
	log.Printf("Last play type: %s, Should reschedule: %v\n", lastPlay.TypeDescKey, shouldReschedule)

	return ProcessResult{
		ShouldReschedule: shouldReschedule,
		LastPlay:         lastPlay,
		MaxPeriods:       maxPeriods,
	}
}

// RescheduleInterval returns the next-check interval to use for the given last play.
// Period-end events use the extended interval except in regular-season period 3,
// where OT/game-end is imminent and the standard interval is used.
func RescheduleInterval(lastPlay models.Play, maxPeriods *int, cfg *config.Config) time.Duration {
	if lastPlay.TypeDescKey == "period-end" {
		isRegularSeason := maxPeriods != nil
		if isRegularSeason && lastPlay.PeriodDescriptor.Number == 3 {
			return time.Duration(cfg.MessageIntervalSeconds) * time.Second
		}
		return time.Duration(cfg.PeriodEndIntervalSeconds) * time.Second
	}
	return time.Duration(cfg.MessageIntervalSeconds) * time.Second
}

// FormatGameState returns a formatted game state string based on the play data.
// Returns "Final" for game-end, "Shootout" for SO, "X:XX left, OT" for overtime,
// or "X:XX left, Nth period" for regular periods.
func FormatGameState(play models.Play) string {
	if play.TypeDescKey == "game-end" {
		return "Final"
	}

	if play.TimeRemaining == "" {
		return ""
	}

	switch play.PeriodDescriptor.PeriodType {
	case "OT":
		return fmt.Sprintf("%s left, OT", play.TimeRemaining)
	case "SO":
		return "Shootout"
	default:
		periodSuffix := map[int]string{1: "1st", 2: "2nd", 3: "3rd"}
		suffix, ok := periodSuffix[play.PeriodDescriptor.Number]
		if !ok {
			suffix = fmt.Sprintf("%dth", play.PeriodDescriptor.Number)
		}
		return fmt.Sprintf("%s left, %s period", play.TimeRemaining, suffix)
	}
}

// AdjustScoreForShootout increments the winning team's score by 1 when the game
// ends in a shootout. This is the shared implementation used by both handler modes.
func AdjustScoreForShootout(gameData map[string]string) error {
	homeScore, err := strconv.Atoi(gameData["homeTeamGoals"])
	if err != nil {
		return fmt.Errorf("invalid home goals: %w", err)
	}

	awayScore, err := strconv.Atoi(gameData["awayTeamGoals"])
	if err != nil {
		return fmt.Errorf("invalid away goals: %w", err)
	}

	homeSOGoals, err := strconv.Atoi(gameData["homeTeamShootOutGoals"])
	if err != nil {
		return fmt.Errorf("invalid home shootout goals: %w", err)
	}

	awaySOGoals, err := strconv.Atoi(gameData["awayTeamShootOutGoals"])
	if err != nil {
		return fmt.Errorf("invalid away shootout goals: %w", err)
	}

	if homeSOGoals > awaySOGoals {
		homeScore++
	} else if awaySOGoals > homeSOGoals {
		awayScore++
	}

	gameData["homeTeamGoals"] = strconv.Itoa(homeScore)
	gameData["awayTeamGoals"] = strconv.Itoa(awayScore)

	return nil
}
