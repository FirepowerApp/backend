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
			log.Printf("Error parsing executionEnd: %v", err)
		} else if time.Now().After(executionEnd) {
			log.Printf("Max execution time (%s) is set and has been reached. Do not reschedule.", executionEnd.Format(time.RFC3339))
			return false
		} else {
			log.Printf("Max execution time (%s) is set and not reached. Continue exection.", executionEnd.Format(time.RFC3339))
			return true
		}
	} else {
		log.Println("Max execution time not set, proceeding without time check.")
	}

	if lastPlay.TypeDescKey != "game-end" {
		return true
	}

	return false
}
