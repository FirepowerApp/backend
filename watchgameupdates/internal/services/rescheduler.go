package services

import (
	"log"
	"time"
	"watchgameupdates/internal/models"
)

func ShouldReschedule(payload models.Payload, lastPlay models.Play) bool {
	if payload.ExecutionEnd != nil {
		executionEnd, err := time.Parse(time.RFC3339, *payload.ExecutionEnd)
		if err != nil {
			// http.Error(w, "Invalid scheduled_time format", http.StatusBadRequest)
			log.Printf("Error parsing executionEnd: %v", err)
		} else if time.Now().After(executionEnd) {
			log.Printf("Current time is after max execution time (%s). Do not reschedule.", executionEnd.Format(time.RFC3339))
			return false
		} else {
			log.Printf("Current time is before max execution time (%s).", executionEnd.Format(time.RFC3339))
		}
	} else {
		log.Println("Max execution time not set, proceeding without time check.")
	}

	if lastPlay.TypeDescKey != "game-end" {
		return true
	}

	return false
}
