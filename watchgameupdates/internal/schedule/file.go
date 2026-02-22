package schedule

import (
	"context"
	"log"
)

// FileScheduleFetcher reads the schedule from a local JSON file.
type FileScheduleFetcher struct {
	FilePath string
}

func (f *FileScheduleFetcher) FetchSchedule(ctx context.Context, date string) ([]ScheduleGame, error) {
	log.Printf("Reading schedule from file: %s", f.FilePath)

	resp, err := FetchScheduleFromFile(f.FilePath)
	if err != nil {
		return nil, err
	}

	games := filterGamesByDate(*resp, date)
	log.Printf("Found %d games for date %s in file", len(games), date)

	return games, nil
}
