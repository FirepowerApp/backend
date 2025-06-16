package tasks

import (
	"context"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
)

type CloudTasksClient interface {
	CreateTask(ctx context.Context, req *taskspb.CreateTaskRequest) (*taskspb.Task, error)
	Close() error
}

type realClient struct {
	client *cloudtasks.Client
}

func (r *realClient) CreateTask(ctx context.Context, req *taskspb.CreateTaskRequest) (*taskspb.Task, error) {
	return r.client.CreateTask(ctx, req)
}

func (r *realClient) Close() error {
	return r.client.Close()
}
