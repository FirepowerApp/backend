package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"watchgameupdates/config"
	"watchgameupdates/internal/models"
	"watchgameupdates/internal/tasks"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CloudTasksQueue implements GameTaskQueue using Google Cloud Tasks.
type CloudTasksQueue struct {
	client tasks.CloudTasksClient
	cfg    *config.Config
}

// NewCloudTasksQueue creates a new CloudTasksQueue.
func NewCloudTasksQueue(ctx context.Context, cfg *config.Config) (*CloudTasksQueue, error) {
	client, err := tasks.NewCloudTasksClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud tasks client: %w", err)
	}
	return &CloudTasksQueue{client: client, cfg: cfg}, nil
}

func (q *CloudTasksQueue) Enqueue(ctx context.Context, payload models.Payload, deliverAt time.Time) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s",
		q.cfg.ProjectID, q.cfg.LocationID, q.cfg.QueueID)

	task := &taskspb.Task{
		MessageType: &taskspb.Task_HttpRequest{
			HttpRequest: &taskspb.HttpRequest{
				HttpMethod: taskspb.HttpMethod_POST,
				Url:        q.cfg.HandlerAddress,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: payloadJSON,
			},
		},
		ScheduleTime: timestamppb.New(deliverAt),
	}

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task:   task,
	}

	log.Printf("Enqueuing task for game %s (%s vs %s) scheduled at %s",
		payload.Game.ID,
		payload.Game.AwayTeam.Abbrev,
		payload.Game.HomeTeam.Abbrev,
		deliverAt.Format(time.RFC3339))

	_, err = q.client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

func (q *CloudTasksQueue) Close() error {
	return q.client.Close()
}
