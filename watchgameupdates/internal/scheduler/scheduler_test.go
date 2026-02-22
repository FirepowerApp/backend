package scheduler

import (
	"context"
	"testing"
	"time"

	"watchgameupdates/internal/models"
	"watchgameupdates/internal/schedule"
)

// mockQueue records enqueued tasks for testing.
type mockQueue struct {
	tasks []enqueuedTask
}

type enqueuedTask struct {
	payload   models.Payload
	deliverAt time.Time
}

func (q *mockQueue) Enqueue(_ context.Context, payload models.Payload, deliverAt time.Time) error {
	q.tasks = append(q.tasks, enqueuedTask{payload: payload, deliverAt: deliverAt})
	return nil
}

func (q *mockQueue) Close() error { return nil }

// mockFetcher returns a fixed set of games.
type mockFetcher struct {
	games []schedule.ScheduleGame
	err   error
}

func (f *mockFetcher) FetchSchedule(_ context.Context, _ string) ([]schedule.ScheduleGame, error) {
	return f.games, f.err
}

func TestScheduler_Run_SchedulesFutureGames(t *testing.T) {
	futureTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	games := []schedule.ScheduleGame{
		{
			ID:           2025020001,
			GameDate:     "2025-10-08",
			StartTimeUTC: futureTime,
			GameState:    "FUT",
			HomeTeam:     models.Team{Abbrev: "TOR", ID: 10},
			AwayTeam:     models.Team{Abbrev: "MTL", ID: 8},
		},
		{
			ID:           2025020002,
			GameDate:     "2025-10-08",
			StartTimeUTC: futureTime,
			GameState:    "FUT",
			HomeTeam:     models.Team{Abbrev: "BOS", ID: 6},
			AwayTeam:     models.Team{Abbrev: "NYR", ID: 3},
		},
	}

	q := &mockQueue{}
	fetcher := &mockFetcher{games: games}
	s := New(fetcher, q, 5, true)

	err := s.Run(context.Background(), "2025-10-08")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(q.tasks))
	}

	// Verify first task
	if q.tasks[0].payload.Game.ID != "2025020001" {
		t.Errorf("expected game ID 2025020001, got %s", q.tasks[0].payload.Game.ID)
	}
	if q.tasks[0].payload.Game.AwayTeam.Abbrev != "MTL" {
		t.Errorf("expected away team MTL, got %s", q.tasks[0].payload.Game.AwayTeam.Abbrev)
	}
	if q.tasks[0].payload.ExecutionEnd == nil {
		t.Error("expected ExecutionEnd to be set")
	}
	if q.tasks[0].payload.ShouldNotify == nil || !*q.tasks[0].payload.ShouldNotify {
		t.Error("expected ShouldNotify to be true")
	}
}

func TestScheduler_Run_SkipsNonFutureGames(t *testing.T) {
	futureTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	games := []schedule.ScheduleGame{
		{
			ID:           2025020001,
			GameDate:     "2025-10-08",
			StartTimeUTC: futureTime,
			GameState:    "FUT",
			HomeTeam:     models.Team{Abbrev: "TOR", ID: 10},
			AwayTeam:     models.Team{Abbrev: "MTL", ID: 8},
		},
		{
			ID:           2025020002,
			GameDate:     "2025-10-08",
			StartTimeUTC: futureTime,
			GameState:    "LIVE",
			HomeTeam:     models.Team{Abbrev: "BOS", ID: 6},
			AwayTeam:     models.Team{Abbrev: "NYR", ID: 3},
		},
		{
			ID:           2025020003,
			GameDate:     "2025-10-08",
			StartTimeUTC: futureTime,
			GameState:    "OFF",
			HomeTeam:     models.Team{Abbrev: "VAN", ID: 23},
			AwayTeam:     models.Team{Abbrev: "EDM", ID: 22},
		},
	}

	q := &mockQueue{}
	fetcher := &mockFetcher{games: games}
	s := New(fetcher, q, 5, true)

	err := s.Run(context.Background(), "2025-10-08")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.tasks) != 1 {
		t.Fatalf("expected 1 task (only FUT games), got %d", len(q.tasks))
	}

	if q.tasks[0].payload.Game.ID != "2025020001" {
		t.Errorf("expected game ID 2025020001, got %s", q.tasks[0].payload.Game.ID)
	}
}

func TestScheduler_Run_NoGames(t *testing.T) {
	q := &mockQueue{}
	fetcher := &mockFetcher{games: nil}
	s := New(fetcher, q, 5, true)

	err := s.Run(context.Background(), "2025-07-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.tasks) != 0 {
		t.Fatalf("expected 0 tasks for off-season date, got %d", len(q.tasks))
	}
}

func TestScheduler_Run_ExecutionEndCalculation(t *testing.T) {
	startTime := time.Date(2025, 10, 8, 23, 0, 0, 0, time.UTC)
	games := []schedule.ScheduleGame{
		{
			ID:           2025020001,
			GameDate:     "2025-10-08",
			StartTimeUTC: startTime.Format(time.RFC3339),
			GameState:    "FUT",
			HomeTeam:     models.Team{Abbrev: "TOR", ID: 10},
			AwayTeam:     models.Team{Abbrev: "MTL", ID: 8},
		},
	}

	q := &mockQueue{}
	fetcher := &mockFetcher{games: games}
	maxHours := 5
	s := New(fetcher, q, maxHours, false)

	err := s.Run(context.Background(), "2025-10-08")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(q.tasks))
	}

	// Verify execution end is startTime + maxHours
	expectedEnd := startTime.Add(time.Duration(maxHours) * time.Hour).Format(time.RFC3339)
	if *q.tasks[0].payload.ExecutionEnd != expectedEnd {
		t.Errorf("expected ExecutionEnd %s, got %s", expectedEnd, *q.tasks[0].payload.ExecutionEnd)
	}

	// Verify deliverAt matches start time
	if !q.tasks[0].deliverAt.Equal(startTime) {
		t.Errorf("expected deliverAt %v, got %v", startTime, q.tasks[0].deliverAt)
	}

	// Verify ShouldNotify is false
	if q.tasks[0].payload.ShouldNotify == nil || *q.tasks[0].payload.ShouldNotify {
		t.Error("expected ShouldNotify to be false")
	}
}
