package services

import "watchgameupdates/internal/models"

func ShouldReschedule(payload models.Payload, lastPlay models.Play) bool {
	if lastPlay.TypeDescKey != "game-end" {
		return true
	}
	return false
}
