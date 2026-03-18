package schedule

import (
	"context"
	"log"
	"time"
)

// FileScheduleFetcher reads the schedule from a local JSON file.
// Used for testing - adjusts game times to start immediately.
type FileScheduleFetcher struct {
	FilePath string
}

func (f *FileScheduleFetcher) FetchSchedule(ctx context.Context, date string) ([]ScheduleGame, error) {
	log.Printf("Reading schedule from file: %s", f.FilePath)

	resp, err := FetchScheduleFromFile(f.FilePath)
	if err != nil {
		return nil, err
	}

	// Collect all games from file, ignoring date structure
	var games []ScheduleGame
	for _, day := range resp.GameWeek {
		games = append(games, day.Games...)
	}

	// Adjust game times for testing: schedule each game to start soon
	// with 10-second intervals between games
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	for i := range games {
		startTime := now.Add(time.Duration(i*10+60) * time.Second) // 60s + 10s per game
		games[i].GameDate = today
		games[i].StartTimeUTC = startTime.Format(time.RFC3339)
		log.Printf("Adjusted game %d start time to %s", games[i].ID, games[i].StartTimeUTC)
	}

	log.Printf("Found %d games in file (adjusted for immediate testing)", len(games))

	return games, nil
}
