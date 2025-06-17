package models

type Payload struct {
	GameID       string  `json:"game_id"`
	ExecutionEnd *string `json:"execution_end,omitempty"`
}
