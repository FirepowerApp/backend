package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/services"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
)

// fakeCloudTasksClient records CreateTask calls for assertions, satisfying
// tasks.CloudTasksClient without touching a real (or emulated) Cloud Tasks
// backend.
type fakeCloudTasksClient struct {
	mu       sync.Mutex
	created  []*taskspb.CreateTaskRequest
	closed   bool
	closeErr error
}

func (f *fakeCloudTasksClient) CreateTask(_ context.Context, req *taskspb.CreateTaskRequest) (*taskspb.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, req)
	return &taskspb.Task{Name: req.GetParent() + "/tasks/fake"}, nil
}

func (f *fakeCloudTasksClient) Close() error {
	f.closed = true
	return f.closeErr
}

func (f *fakeCloudTasksClient) taskCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.created)
}

// TestScheduleNextCheckWithClient_PreservesDataSource is the CRITICAL
// regression test for the actually-deployed Cloud Tasks HTTP path (see
// cmd/watchgameupdates -mode=http, the default). internal/tasks has an
// equivalent test for the alternate asynq/Redis worker mode
// (TestProcessTask_RescheduledTaskPreservesDataSource), but that path is not
// what's running in staging/production — this one is.
func TestScheduleNextCheckWithClient_PreservesDataSource(t *testing.T) {
	client := &fakeCloudTasksClient{}
	cfg := &config.Config{ProjectID: "proj", LocationID: "loc", QueueID: "queue", HandlerAddress: "http://backend:8080"}

	payload := models.Payload{
		Game:       models.Game{ID: "2024030411"},
		DataSource: "emulator",
	}

	err := scheduleNextCheckWithClient(context.Background(), client, cfg, payload, 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.taskCount() != 1 {
		t.Fatalf("expected 1 created task, got %d", client.taskCount())
	}

	body := client.created[0].GetTask().GetHttpRequest().GetBody()
	var parsed models.Payload
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to unmarshal reschedule task body: %v", err)
	}
	if parsed.DataSource != "emulator" {
		t.Errorf("DataSource did not survive reschedule: got %q, want %q", parsed.DataSource, "emulator")
	}
}

func TestScheduleNextCheckWithClient_CreateTaskErrorPropagates(t *testing.T) {
	cfg := &config.Config{ProjectID: "proj", LocationID: "loc", QueueID: "queue", HandlerAddress: "http://backend:8080"}
	client := &erroringCloudTasksClient{}

	err := scheduleNextCheckWithClient(context.Background(), client, cfg, models.Payload{Game: models.Game{ID: "1"}}, time.Second)
	if err == nil {
		t.Error("expected error when CreateTask fails, got nil")
	}
}

type erroringCloudTasksClient struct{}

func (e *erroringCloudTasksClient) CreateTask(_ context.Context, _ *taskspb.CreateTaskRequest) (*taskspb.Task, error) {
	return nil, context.DeadlineExceeded
}

func (e *erroringCloudTasksClient) Close() error { return nil }

// scheduleNextCheck used to call log.Fatalf when it could not build the Cloud
// Tasks client, which exits the process. The handler is a single replica, so that
// took down tracking for every game in flight. If the Fatalf returns, this test
// kills its own binary instead of failing politely, which is the point.
func TestWatchGameUpdatesHandler_TasksClientErrorReturns500AndSurvives(t *testing.T) {
	// Play-by-play is down, so the handler must reschedule, which builds the client.
	pbp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer pbp.Close()
	t.Setenv("PLAYBYPLAY_API_BASE_URL", pbp.URL)

	// No emulator, and credentials that cannot load: NewCloudTasksClient fails
	// deterministically, whatever the developer's own gcloud login looks like.
	t.Setenv("USE_TASKS_EMULATOR", "false")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/credentials.json")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	WatchGameUpdatesHandler(w, r, &services.HTTPGameDataFetcher{},
		notification.NewServiceWithNotificationFlag(true),
		models.Payload{Game: models.Game{ID: "2025030415"}})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 so Cloud Tasks retries, got %d", w.Code)
	}
}
