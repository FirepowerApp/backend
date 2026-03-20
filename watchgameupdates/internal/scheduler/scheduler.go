package scheduler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"watchgameupdates/internal/models"
	"watchgameupdates/internal/schedule"
)

// TaskEnqueuer is the interface used by the scheduler to enqueue game tasks.
// This is defined here (at the consumer) per Go convention. Concrete
// implementations live in the queue package (Cloud Tasks, future Redis, etc.).
type TaskEnqueuer interface {
	Enqueue(ctx context.Context, payload models.Payload, deliverAt time.Time) error
	Close() error
}

// MessageSender can send a plain-text notification message.
type MessageSender interface {
	SendMessage(ctx context.Context, message string)
}

// Scheduler fetches the NHL schedule and enqueues game tracking tasks.
type Scheduler struct {
	fetcher         schedule.ScheduleFetcher
	queue           TaskEnqueuer
	gameMaxDuration time.Duration
	shouldNotify    bool
	teamFilter      string
	notifier        MessageSender
}

// New creates a new Scheduler.
func New(fetcher schedule.ScheduleFetcher, q TaskEnqueuer, gameMaxDurationHours int, shouldNotify bool, teamFilter string, notifier MessageSender) *Scheduler {
	return &Scheduler{
		fetcher:         fetcher,
		queue:           q,
		gameMaxDuration: time.Duration(gameMaxDurationHours) * time.Hour,
		shouldNotify:    shouldNotify,
		teamFilter:      teamFilter,
		notifier:        notifier,
	}
}

// Run fetches the schedule for the given date and enqueues a task for each future game.
func (s *Scheduler) Run(ctx context.Context, date string) error {
	log.Printf("Fetching schedule for %s", date)

	games, err := s.fetcher.FetchSchedule(ctx, date)
	if err != nil {
		return fmt.Errorf("failed to fetch schedule: %w", err)
	}

	if len(games) == 0 {
		log.Printf("No games found for %s", date)
		return nil
	}

	log.Printf("Found %d games for %s", len(games), date)

	scheduled := 0
	var scheduledGames []schedule.ScheduleGame
	for _, game := range games {
		if s.teamFilter != "" && game.HomeTeam.Abbrev != s.teamFilter && game.AwayTeam.Abbrev != s.teamFilter {
			log.Printf("Skipping game %d (%s vs %s) - neither team matches filter %s",
				game.ID, game.AwayTeam.Abbrev, game.HomeTeam.Abbrev, s.teamFilter)
			continue
		}

		if game.GameState != "FUT" {
			log.Printf("Skipping game %d (%s vs %s) - state is %s, not FUT",
				game.ID, game.AwayTeam.Abbrev, game.HomeTeam.Abbrev, game.GameState)
			continue
		}

		startTime, err := time.Parse(time.RFC3339, game.StartTimeUTC)
		if err != nil {
			log.Printf("Failed to parse start time for game %d: %v", game.ID, err)
			continue
		}

		executionEnd := startTime.Add(s.gameMaxDuration).Format(time.RFC3339)

		payload := models.Payload{
			Game: models.Game{
				ID:        strconv.Itoa(game.ID),
				GameDate:  game.GameDate,
				StartTime: game.StartTimeUTC,
				HomeTeam:  game.HomeTeam,
				AwayTeam:  game.AwayTeam,
			},
			ExecutionEnd: &executionEnd,
			ShouldNotify: &s.shouldNotify,
		}

		if err := s.queue.Enqueue(ctx, payload, startTime); err != nil {
			log.Printf("Failed to enqueue task for game %d: %v", game.ID, err)
			continue
		}

		scheduledGames = append(scheduledGames, game)
		scheduled++
	}

	log.Printf("Successfully scheduled %d/%d tasks for %s", scheduled, len(games), date)

	if scheduled > 0 && s.notifier != nil {
		s.notifier.SendMessage(ctx, formatSchedulerSummary(date, scheduledGames))
	}

	return nil
}

// formatSchedulerSummary builds a Discord message summarising the scheduled games.
func formatSchedulerSummary(date string, games []schedule.ScheduleGame) string {
	msg := fmt.Sprintf("🏒 Scheduled %d game(s) for %s:\n", len(games), date)
	for _, g := range games {
		startTime, err := time.Parse(time.RFC3339, g.StartTimeUTC)
		if err != nil {
			msg += fmt.Sprintf("• %s @ %s\n", g.AwayTeam.Abbrev, g.HomeTeam.Abbrev)
			continue
		}
		msg += fmt.Sprintf("• %s @ %s — %s UTC\n", g.AwayTeam.Abbrev, g.HomeTeam.Abbrev, startTime.UTC().Format("3:04 PM"))
	}
	return msg
}
