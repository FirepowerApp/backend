package models

// Clock is the game clock the NHL API reports at the top level of the response.
// It is fresher than the last play's timeRemaining and states intermissions
// directly instead of leaving callers to infer them from play types.
type Clock struct {
	TimeRemaining  string `json:"timeRemaining"`
	InIntermission bool   `json:"inIntermission"`
}

// TeamState carries the official score, which already includes a shootout winner.
type TeamState struct {
	Score int `json:"score"`
}

// PlayByPlayResponse is the NHL play-by-play document. The game-level fields are
// the authority for state, clock and score; Plays is only used for the event type.
type PlayByPlayResponse struct {
	GameType         int              `json:"gameType"`  // 2 = regular season, 3 = playoffs
	GameState        string           `json:"gameState"` // FUT|PRE|LIVE|CRIT|OFF|FINAL
	PeriodDescriptor PeriodDescriptor `json:"periodDescriptor"`
	Clock            Clock            `json:"clock"`
	HomeTeam         TeamState        `json:"homeTeam"`
	AwayTeam         TeamState        `json:"awayTeam"`
	Plays            []Play           `json:"plays"`
}
