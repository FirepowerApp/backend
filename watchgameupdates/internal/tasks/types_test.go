package tasks

import (
	"testing"

	"watchgameupdates/internal/models"

	"github.com/hibiken/asynq"
)

func TestNewWatchGameUpdatesTask(t *testing.T) {
	t.Run("ValidPayload", func(t *testing.T) {
		execEnd := "2025-01-01T12:00:00Z"
		notify := true
		payload := models.Payload{
			Game: models.Game{
				ID:       "2024030411",
				GameDate: "2025-01-01",
				HomeTeam: models.Team{Abbrev: "CHI"},
				AwayTeam: models.Team{Abbrev: "DET"},
			},
			ExecutionEnd: &execEnd,
			ShouldNotify: &notify,
		}

		task, err := NewWatchGameUpdatesTask(payload)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if task.Type() != TypeWatchGameUpdates {
			t.Errorf("Expected task type %q, got %q", TypeWatchGameUpdates, task.Type())
		}

		if len(task.Payload()) == 0 {
			t.Error("Expected non-empty payload")
		}
	})

	t.Run("MinimalPayload", func(t *testing.T) {
		payload := models.Payload{
			Game: models.Game{ID: "2024030411"},
		}

		task, err := NewWatchGameUpdatesTask(payload)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if task.Type() != TypeWatchGameUpdates {
			t.Errorf("Expected task type %q, got %q", TypeWatchGameUpdates, task.Type())
		}
	})
}

func TestParseWatchGameUpdatesPayload(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		execEnd := "2025-01-01T12:00:00Z"
		notify := false
		original := models.Payload{
			Game: models.Game{
				ID:       "2024030411",
				GameDate: "2025-01-01",
				HomeTeam: models.Team{Abbrev: "CHI"},
				AwayTeam: models.Team{Abbrev: "DET"},
			},
			ExecutionEnd: &execEnd,
			ShouldNotify: &notify,
		}

		task, err := NewWatchGameUpdatesTask(original)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		parsed, err := ParseWatchGameUpdatesPayload(task)
		if err != nil {
			t.Fatalf("Failed to parse payload: %v", err)
		}

		if parsed.Game.ID != original.Game.ID {
			t.Errorf("Game ID mismatch: got %q, want %q", parsed.Game.ID, original.Game.ID)
		}
		if parsed.Game.GameDate != original.Game.GameDate {
			t.Errorf("GameDate mismatch: got %q, want %q", parsed.Game.GameDate, original.Game.GameDate)
		}
		if parsed.Game.HomeTeam.Abbrev != original.Game.HomeTeam.Abbrev {
			t.Errorf("HomeTeam.Abbrev mismatch: got %q, want %q", parsed.Game.HomeTeam.Abbrev, original.Game.HomeTeam.Abbrev)
		}
		if parsed.Game.AwayTeam.Abbrev != original.Game.AwayTeam.Abbrev {
			t.Errorf("AwayTeam.Abbrev mismatch: got %q, want %q", parsed.Game.AwayTeam.Abbrev, original.Game.AwayTeam.Abbrev)
		}
		if parsed.ExecutionEnd == nil || *parsed.ExecutionEnd != execEnd {
			t.Errorf("ExecutionEnd mismatch: got %v, want %v", parsed.ExecutionEnd, &execEnd)
		}
		if parsed.ShouldNotify == nil || *parsed.ShouldNotify != notify {
			t.Errorf("ShouldNotify mismatch: got %v, want %v", parsed.ShouldNotify, &notify)
		}
	})

	t.Run("NilOptionalFields", func(t *testing.T) {
		original := models.Payload{
			Game: models.Game{ID: "2024030411"},
		}

		task, err := NewWatchGameUpdatesTask(original)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		parsed, err := ParseWatchGameUpdatesPayload(task)
		if err != nil {
			t.Fatalf("Failed to parse payload: %v", err)
		}

		if parsed.ExecutionEnd != nil {
			t.Errorf("Expected nil ExecutionEnd, got %v", parsed.ExecutionEnd)
		}
		if parsed.ShouldNotify != nil {
			t.Errorf("Expected nil ShouldNotify, got %v", parsed.ShouldNotify)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		// Create a task with invalid JSON payload
		task := newTaskWithPayload(TypeWatchGameUpdates, []byte("not-valid-json"))

		_, err := ParseWatchGameUpdatesPayload(task)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})
}

// newTaskWithPayload creates an asynq.Task for testing with an arbitrary payload.
func newTaskWithPayload(typeName string, payload []byte) *asynq.Task {
	return asynq.NewTask(typeName, payload)
}
