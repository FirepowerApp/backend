package services

import (
	"fmt"
	"log"
	"strconv"
	"time"

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
	LastPlayType     string
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
	}

	lastPlay := FetchPlayByPlay(payload.Game.ID)

	if _, ok := recomputeTypes[lastPlay.TypeDescKey]; ok {
		log.Printf("Processing play type '%s' for game %s - fetching MoneyPuck data", lastPlay.TypeDescKey, payload.Game.ID)

		requiredKeys := gp.NotificationService.GetAllRequiredDataKeys()
		gameData, err := gp.Fetcher.FetchAndParseGameData(payload.Game.ID, requiredKeys)

		if lastPlay.TypeDescKey == "game-end" && gameData != nil {
			homeGoals, homeGOK := gameData["homeTeamGoals"]
			awayGoals, awayGOK := gameData["awayTeamGoals"]
			if homeGOK && awayGOK && homeGoals == awayGoals {
				if shootoutErr := AdjustScoreForShootout(gameData); shootoutErr != nil {
					log.Printf("Failed to adjust score for shootout: %v", shootoutErr)
				}
			}
		}

		if err != nil {
			log.Printf("ERROR: Failed to fetch and parse MoneyPuck data for game %s: %v", payload.Game.ID, err)
		}

		gp.NotificationService.SendGameEventNotifications(payload.Game, gameData)
	}

	shouldReschedule := ShouldReschedule(payload, lastPlay)
	log.Printf("Last play type: %s, Should reschedule: %v\n", lastPlay.TypeDescKey, shouldReschedule)

	return ProcessResult{
		ShouldReschedule: shouldReschedule,
		LastPlayType:     lastPlay.TypeDescKey,
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
