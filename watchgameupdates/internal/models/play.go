package models

type PeriodDescriptor struct {
	Number     int    `json:"number"`
	PeriodType string `json:"periodType"`
}

type Play struct {
	TypeDescKey      string           `json:"typeDescKey"`
	Period           int              `json:"period"`
	PeriodDescriptor PeriodDescriptor `json:"periodDescriptor"`
	TimeInPeriod     string           `json:"timeInPeriod"`
	TimeRemaining    string           `json:"timeRemaining"`
}
