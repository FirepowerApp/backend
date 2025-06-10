package services

import "watchgameupdates/internal/models"

func ShouldReschedule(payload models.Payload, lastPlay models.Play) bool {
	if payload.ForceReschedule != nil && *payload.ForceReschedule {
		return true
	}
	if lastPlay.TypeDescKey != "game-end" {
		return true
	}
	return false
}
