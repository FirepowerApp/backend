package tasks

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"

	"github.com/hibiken/asynq"
)

// mockEnqueuer captures enqueued tasks for assertions.
type mockEnqueuer struct {
	mu       sync.Mutex
	enqueued []enqueuedTask
	err      error
}

type enqueuedTask struct {
	task *asynq.Task
	opts []asynq.Option
}

func (m *mockEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return nil, m.err
	}

	m.enqueued = append(m.enqueued, enqueuedTask{task: task, opts: opts})
	return &asynq.TaskInfo{
		ID:    "test-task-id",
		Queue: "default",
	}, nil
}

func (m *mockEnqueuer) taskCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.enqueued)
}

func TestProcessTask_InvalidPayload(t *testing.T) {
	cfg := &config.Config{MessageIntervalSeconds: 60}
	enqueuer := &mockEnqueuer{}
	h := NewWatchGameUpdatesHandler(cfg, enqueuer)

	task := asynq.NewTask(TypeWatchGameUpdates, []byte("invalid-json"))

	err := h.ProcessTask(context.Background(), task)
	if err == nil {
		t.Error("Expected error for invalid payload, got nil")
	}
}

func TestProcessTask_ExpiredExecutionWindow(t *testing.T) {
	cfg := &config.Config{MessageIntervalSeconds: 60}
	enqueuer := &mockEnqueuer{}
	h := NewWatchGameUpdatesHandler(cfg, enqueuer)

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	payload := models.Payload{
		Game:         models.Game{ID: "2024030411"},
		ExecutionEnd: &past,
	}

	data, _ := json.Marshal(payload)
	task := asynq.NewTask(TypeWatchGameUpdates, data)

	err := h.ProcessTask(context.Background(), task)
	if err != nil {
		t.Errorf("Expected no error for expired window, got: %v", err)
	}

	// Should NOT enqueue a follow-up task
	if enqueuer.taskCount() != 0 {
		t.Errorf("Expected 0 enqueued tasks for expired window, got %d", enqueuer.taskCount())
	}
}

func TestProcessTask_InvalidExecutionEndFormat(t *testing.T) {
	cfg := &config.Config{MessageIntervalSeconds: 60}
	enqueuer := &mockEnqueuer{}
	h := NewWatchGameUpdatesHandler(cfg, enqueuer)

	invalid := "not-a-date"
	payload := models.Payload{
		Game:         models.Game{ID: "2024030411"},
		ExecutionEnd: &invalid,
	}

	data, _ := json.Marshal(payload)
	task := asynq.NewTask(TypeWatchGameUpdates, data)

	err := h.ProcessTask(context.Background(), task)
	if err == nil {
		t.Error("Expected error for invalid execution end format, got nil")
	}
}

func TestNewWatchGameUpdatesHandler_NotNil(t *testing.T) {
	cfg := &config.Config{MessageIntervalSeconds: 60, RedisAddress: "localhost:6379"}
	enqueuer := &mockEnqueuer{}

	h := NewWatchGameUpdatesHandler(cfg, enqueuer)
	if h == nil {
		t.Error("Expected non-nil handler")
	}
	if h.cfg != cfg {
		t.Error("Handler config mismatch")
	}
	if h.enqueuer != enqueuer {
		t.Error("Handler enqueuer mismatch")
	}
}

func TestScheduleNextCheck_EnqueuesCalled(t *testing.T) {
	cfg := &config.Config{MessageIntervalSeconds: 30}
	enqueuer := &mockEnqueuer{}
	h := NewWatchGameUpdatesHandler(cfg, enqueuer)

	payload := models.Payload{
		Game: models.Game{ID: "2024030411"},
	}

	err := h.scheduleNextCheck(payload)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if enqueuer.taskCount() != 1 {
		t.Errorf("Expected 1 enqueued task, got %d", enqueuer.taskCount())
	}

	// Verify the enqueued task has the correct type
	enqueuer.mu.Lock()
	defer enqueuer.mu.Unlock()
	if enqueuer.enqueued[0].task.Type() != TypeWatchGameUpdates {
		t.Errorf("Expected task type %q, got %q", TypeWatchGameUpdates, enqueuer.enqueued[0].task.Type())
	}

	// Verify the payload round-trips correctly
	parsed, err := ParseWatchGameUpdatesPayload(enqueuer.enqueued[0].task)
	if err != nil {
		t.Fatalf("Failed to parse enqueued task payload: %v", err)
	}
	if parsed.Game.ID != "2024030411" {
		t.Errorf("Expected game ID %q in enqueued task, got %q", "2024030411", parsed.Game.ID)
	}
}

func TestScheduleNextCheck_EnqueueError(t *testing.T) {
	cfg := &config.Config{MessageIntervalSeconds: 30}
	enqueuer := &mockEnqueuer{err: asynq.ErrDuplicateTask}
	h := NewWatchGameUpdatesHandler(cfg, enqueuer)

	payload := models.Payload{
		Game: models.Game{ID: "2024030411"},
	}

	err := h.scheduleNextCheck(payload)
	if err == nil {
		t.Error("Expected error when enqueue fails, got nil")
	}
}
