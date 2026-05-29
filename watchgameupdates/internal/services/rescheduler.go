package services

import (
	"log/slog"
	"time"
	"watchgameupdates/internal/models"
)

func ShouldReschedule(payload models.Payload, lastPlay models.Play) bool {
	if payload.ExecutionEnd != nil {
		executionEnd, err := time.Parse(time.RFC3339, *payload.ExecutionEnd)
		if err != nil {
			slog.Error("error parsing executionEnd", "error", err)
		} else if time.Now().After(executionEnd) {
			slog.Info("execution end reached, not rescheduling", "execution_end", executionEnd.Format(time.RFC3339))
			return false
		}
	} else {
		slog.Debug("no execution end set, proceeding without time check")
	}

	if lastPlay.TypeDescKey != "game-end" {
		return true
	}

	return false
}
