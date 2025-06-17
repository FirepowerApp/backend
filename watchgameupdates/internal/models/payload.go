package models

type Payload struct {
	GameID           string  `json:"game_id"`
	MaxExecutionTime *string `json:"max_execution_time,omitempty"`
}
