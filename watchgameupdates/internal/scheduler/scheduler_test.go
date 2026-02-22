package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"watchgameupdates/internal/models"
	"watchgameupdates/internal/schedule"
)

// mockQueue records enqueued tasks for testing.
type mockQueue struct {
	tasks   []enqueuedTask
	failOn  int // fail on the Nth call (0 = never fail)
	callNum int
}

type enqueuedTask struct {
	payload   models.Payload
	deliverAt time.Time
}

func (q *mockQueue) Enqueue(_ context.Context, payload models.Payload, deliverAt time.Time) error {
	q.callNum++
	if q.failOn > 0 && q.callNum == q.failOn {
		return fmt.Errorf("simulated enqueue failure")
	}
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

	// Verify second task
	if q.tasks[1].payload.Game.ID != "2025020002" {
		t.Errorf("expected game ID 2025020002, got %s", q.tasks[1].payload.Game.ID)
	}
	if q.tasks[1].payload.Game.HomeTeam.Abbrev != "BOS" {
		t.Errorf("expected home team BOS, got %s", q.tasks[1].payload.Game.HomeTeam.Abbrev)
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

	// Verify game data is correctly mapped
	if q.tasks[0].payload.Game.StartTime != startTime.Format(time.RFC3339) {
		t.Errorf("expected StartTime %s, got %s", startTime.Format(time.RFC3339), q.tasks[0].payload.Game.StartTime)
	}
	if q.tasks[0].payload.Game.GameDate != "2025-10-08" {
		t.Errorf("expected GameDate 2025-10-08, got %s", q.tasks[0].payload.Game.GameDate)
	}
}

func TestScheduler_Run_FetcherError(t *testing.T) {
	q := &mockQueue{}
	fetcher := &mockFetcher{err: fmt.Errorf("NHL API unavailable")}
	s := New(fetcher, q, 5, true)

	err := s.Run(context.Background(), "2025-10-08")
	if err == nil {
		t.Fatal("expected error from fetcher, got nil")
	}

	if len(q.tasks) != 0 {
		t.Errorf("expected 0 tasks when fetcher fails, got %d", len(q.tasks))
	}
}

func TestScheduler_Run_EnqueueErrorContinues(t *testing.T) {
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

	// Fail on the first enqueue, succeed on the second
	q := &mockQueue{failOn: 1}
	fetcher := &mockFetcher{games: games}
	s := New(fetcher, q, 5, true)

	err := s.Run(context.Background(), "2025-10-08")
	if err != nil {
		t.Fatalf("Run should not return error when individual enqueues fail: %v", err)
	}

	// Only the second game should have been enqueued
	if len(q.tasks) != 1 {
		t.Fatalf("expected 1 task (second game after first failed), got %d", len(q.tasks))
	}

	if q.tasks[0].payload.Game.ID != "2025020002" {
		t.Errorf("expected game ID 2025020002, got %s", q.tasks[0].payload.Game.ID)
	}
}

func TestScheduler_Run_InvalidStartTime(t *testing.T) {
	games := []schedule.ScheduleGame{
		{
			ID:           2025020001,
			GameDate:     "2025-10-08",
			StartTimeUTC: "not-a-valid-time",
			GameState:    "FUT",
			HomeTeam:     models.Team{Abbrev: "TOR", ID: 10},
			AwayTeam:     models.Team{Abbrev: "MTL", ID: 8},
		},
	}

	q := &mockQueue{}
	fetcher := &mockFetcher{games: games}
	s := New(fetcher, q, 5, true)

	err := s.Run(context.Background(), "2025-10-08")
	if err != nil {
		t.Fatalf("Run should not return error for invalid start time: %v", err)
	}

	if len(q.tasks) != 0 {
		t.Fatalf("expected 0 tasks for game with invalid start time, got %d", len(q.tasks))
	}
}

func TestScheduler_Run_GameIDConversion(t *testing.T) {
	futureTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	games := []schedule.ScheduleGame{
		{
			ID:           2025020999,
			GameDate:     "2025-10-08",
			StartTimeUTC: futureTime,
			GameState:    "FUT",
			HomeTeam:     models.Team{Abbrev: "TOR", ID: 10},
			AwayTeam:     models.Team{Abbrev: "MTL", ID: 8},
		},
	}

	q := &mockQueue{}
	fetcher := &mockFetcher{games: games}
	s := New(fetcher, q, 5, true)

	s.Run(context.Background(), "2025-10-08")

	// Verify int→string conversion of game ID
	if q.tasks[0].payload.Game.ID != "2025020999" {
		t.Errorf("expected game ID string '2025020999', got '%s'", q.tasks[0].payload.Game.ID)
	}
}
