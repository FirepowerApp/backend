package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"watchgameupdates/internal/models"
)

func FetchPlayByPlay(gameID string) (lastPlay models.Play, maxPeriods *int) {
	// Get play-by-play API base URL from environment variable
	playByPlayAPIBaseURL := os.Getenv("PLAYBYPLAY_API_BASE_URL")
	if playByPlayAPIBaseURL == "" {
		playByPlayAPIBaseURL = "https://api-web.nhle.com" // Default production URL
	}

	playByPlayUrl := fmt.Sprintf("%s/v1/gamecenter/%s/play-by-play", playByPlayAPIBaseURL, gameID)
	resp, err := http.Get(playByPlayUrl)
	if err != nil {
		slog.Error("failed to fetch play-by-play data", "game_id", gameID, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("play-by-play request returned non-200", "game_id", gameID, "status_code", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("failed to read play-by-play response body", "game_id", gameID, "error", err)
		return
	}
	var data models.PlayByPlayResponse
	if err := json.Unmarshal(body, &data); err != nil {
		panic(err)
	}

	if len(data.Plays) == 0 {
		slog.Warn("no plays found for game", "game_id", gameID)
		return
	}

	lastPlay = data.Plays[len(data.Plays)-1]
	slog.Debug("fetched play-by-play", "game_id", gameID, "last_play_type", lastPlay.TypeDescKey, "regular_season", data.MaxPeriods != nil)
	return lastPlay, data.MaxPeriods
}
