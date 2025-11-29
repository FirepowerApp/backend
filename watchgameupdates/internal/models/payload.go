package models

type Team struct {
	ID                       int               `json:"id"`
	CommonName               map[string]string `json:"commonName"`
	PlaceName                map[string]string `json:"placeName"`
	PlaceNameWithPreposition map[string]string `json:"placeNameWithPreposition"`
	Abbrev                   string            `json:"abbrev"`
}

type Game struct {
	ID        string `json:"id"`
	GameDate  string `json:"gameDate"`
	StartTime string `json:"startTimeUTC"`
	HomeTeam  Team   `json:"homeTeam"`
	AwayTeam  Team   `json:"awayTeam"`
}

type Payload struct {
	Game         Game    `json:"game"`
	ExecutionEnd *string `json:"execution_end,omitempty"`
	ShouldNotify *bool   `json:"should_notify,omitempty"`
}
