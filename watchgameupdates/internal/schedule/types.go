package schedule

import "watchgameupdates/internal/models"

// ScheduleResponse represents the NHL API response from /v1/schedule/{date}.
type ScheduleResponse struct {
	GameWeek []GameWeekDay `json:"gameWeek"`
}

// GameWeekDay represents a single day within the gameWeek array.
type GameWeekDay struct {
	Date  string         `json:"date"`
	Games []ScheduleGame `json:"games"`
}

// ScheduleGame represents a single game in the NHL schedule response.
type ScheduleGame struct {
	ID           int         `json:"id"`
	GameDate     string      `json:"gameDate"`
	StartTimeUTC string      `json:"startTimeUTC"`
	GameState    string      `json:"gameState"`
	GameType     int         `json:"gameType"`
	HomeTeam     models.Team `json:"homeTeam"`
	AwayTeam     models.Team `json:"awayTeam"`
}
