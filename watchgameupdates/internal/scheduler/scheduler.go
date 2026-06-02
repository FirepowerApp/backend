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

const (
	gameStateFUT  = "FUT"
	gameStatePRE  = "PRE"
	gameStateLIVE = "LIVE"
)

// Scheduler fetches the NHL schedule and enqueues game tracking tasks.
type Scheduler struct {
	fetcher          schedule.ScheduleFetcher
	queue            TaskEnqueuer
	gameMaxDuration  time.Duration
	shouldNotify     bool
	teamFilters      []string // empty = monitor all games
	notifier         MessageSender
	includeLiveGames bool
}

// New creates a new Scheduler.
func New(fetcher schedule.ScheduleFetcher, q TaskEnqueuer, gameMaxDurationHours int, shouldNotify bool, teamFilters []string, notifier MessageSender, includeLiveGames bool) *Scheduler {
	return &Scheduler{
		fetcher:          fetcher,
		queue:            q,
		gameMaxDuration:  time.Duration(gameMaxDurationHours) * time.Hour,
		shouldNotify:     shouldNotify,
		teamFilters:      teamFilters,
		notifier:         notifier,
		includeLiveGames: includeLiveGames,
	}
}

// Run fetches the schedule for the given date and enqueues a task for each future game.
func (s *Scheduler) Run(ctx context.Context, date string) error {
	if len(s.teamFilters) == 0 {
		log.Printf("Monitoring ALL teams")
	} else {
		log.Printf("Monitoring teams: %v", s.teamFilters)
	}

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
		if len(s.teamFilters) > 0 && !containsTeam(s.teamFilters, game.HomeTeam.Abbrev) && !containsTeam(s.teamFilters, game.AwayTeam.Abbrev) {
			log.Printf("Skipping game %d (%s vs %s) - neither team in roster %v",
				game.ID, game.AwayTeam.Abbrev, game.HomeTeam.Abbrev, s.teamFilters)
			continue
		}

		isLive := game.GameState == gameStateLIVE
		if game.GameState != gameStateFUT && game.GameState != gameStatePRE && !isLive {
			log.Printf("Skipping game %d (%s vs %s) - state is %s, not FUT or PRE",
				game.ID, game.AwayTeam.Abbrev, game.HomeTeam.Abbrev, game.GameState)
			continue
		}
		if isLive && !s.includeLiveGames {
			log.Printf("Skipping game %d (%s vs %s) - state is LIVE (set INCLUDE_LIVE_GAMES=true to monitor)",
				game.ID, game.AwayTeam.Abbrev, game.HomeTeam.Abbrev)
			continue
		}

		startTime, err := time.Parse(time.RFC3339, game.StartTimeUTC)
		if err != nil {
			log.Printf("Failed to parse start time for game %d: %v", game.ID, err)
			continue
		}

		// For live games, deliver immediately and set executionEnd from now.
		deliverAt := startTime
		executionEndBase := startTime
		if isLive {
			now := time.Now()
			deliverAt = now
			executionEndBase = now
		}
		executionEnd := executionEndBase.Add(s.gameMaxDuration).Format(time.RFC3339)
		shouldNotify := s.shouldNotify

		payload := models.Payload{
			Game: models.Game{
				ID:        strconv.Itoa(game.ID),
				GameDate:  game.GameDate,
				StartTime: game.StartTimeUTC,
				HomeTeam:  game.HomeTeam,
				AwayTeam:  game.AwayTeam,
			},
			ExecutionEnd: &executionEnd,
			ShouldNotify: &shouldNotify,
		}

		if err := s.queue.Enqueue(ctx, payload, deliverAt); err != nil {
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

// containsTeam reports whether abbrev is in the roster slice.
func containsTeam(roster []string, abbrev string) bool {
	for _, t := range roster {
		if t == abbrev {
			return true
		}
	}
	return false
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
