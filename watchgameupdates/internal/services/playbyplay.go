package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"watchgameupdates/internal/models"
)

// upstreamHTTPTimeout bounds every call to an upstream (NHL, MoneyPuck). The
// default client has no deadline, so a half-open connection would park a game
// check forever. Matches apnsHTTPTimeout in the liveactivity package.
const upstreamHTTPTimeout = 10 * time.Second

var httpClient = &http.Client{Timeout: upstreamHTTPTimeout}

// FetchPlayByPlay fetches the play-by-play document for gameID. dataSource (see
// internal/season.DataSource) selects live vs. the staging emulator; empty
// string is treated as live. Any failure is returned, never swallowed, so the
// caller can tell "upstream is down" from "the game has no plays yet".
func FetchPlayByPlay(gameID string, dataSource string) (*models.PlayByPlayResponse, error) {
	url := fmt.Sprintf("%s/v1/gamecenter/%s/play-by-play", resolvePlayByPlayBaseURL(dataSource), gameID)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch play-by-play for game %s: %w", gameID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch play-by-play for game %s: status %d", gameID, resp.StatusCode)
	}

	var data models.PlayByPlayResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode play-by-play for game %s: %w", gameID, err)
	}
	return &data, nil
}
