package models

type Payload struct {
	GameID          string `json:"game_id"`
	ForceReschedule *bool  `json:"force_reschedule,omitempty"`
}
