package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"watchgameupdates/internal/models"
)

func FetchPlayByPlay(gameID string) (lastPlay models.Play) {
	playByPlayUrl := fmt.Sprintf("https://api-web.nhle.com/v1/gamecenter/%s/play-by-play", gameID)
	resp, err := http.Get(playByPlayUrl)
	if err != nil {
		log.Printf("Failed to fetch play-by-play data: %v", err)
		// http.Error(w, "Failed to fetch play-by-play data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to fetch play-by-play data, status code: %d", resp.StatusCode)
		// http.Error(w, "Failed to fetch play-by-play data", http.StatusInternalServerError)
		return
	}

	// Display respnse body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		// http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}
	var data models.PlayByPlayResponse
	if err := json.Unmarshal(body, &data); err != nil {
		panic(err)
	}

	if len(data.Plays) == 0 {
		log.Printf("No plays found for GameID: %s", gameID)
		// http.Error(w, "No plays found for the game", http.StatusNotFound)
		return
	}

	lastPlay = data.Plays[len(data.Plays)-1]
	log.Printf("Last play type: %s", lastPlay.TypeDescKey)
	return lastPlay
}
