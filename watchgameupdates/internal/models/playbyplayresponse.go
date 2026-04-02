package models

type PlayByPlayResponse struct {
	Plays      []Play `json:"plays"`
	MaxPeriods *int   `json:"maxPeriods,omitempty"`
}
