package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	taskspb "google.golang.org/genproto/googleapis/cloud/tasks/v2"
	"google.golang.org/grpc"
)

func createQueue(client taskspb.CloudTasksClient, ctx context.Context) error {
	req := &taskspb.CreateQueueRequest{
		Parent: "projects/localproject/locations/us-south1",
		Queue: &taskspb.Queue{
			Name: "projects/localproject/locations/us-south1/queues/gameschedule",
		},
	}
	_, err := client.CreateQueue(ctx, req)
	return err
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:8123", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := taskspb.NewCloudTasksClient(conn)

	if err := createQueue(client, ctx); err != nil {
		if err.Error() != "rpc error: code = AlreadyExists desc = Queue already exists" {
			log.Fatalf("Failed to create queue: %v", err)
		} else {
			log.Println("Queue already exists, skipping creation")
		}
	}

	gameID := "2024030411"

	type TaskPayload struct {
		GameID       string  `json:"game_id"`
		ExecutionEnd *string `json:"execution_end,omitempty"`
	}

	executionEnd := time.Now().Add(12 * time.Minute).Format(time.RFC3339)
	payload := TaskPayload{
		GameID:       gameID,
		ExecutionEnd: &executionEnd,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	// Cloud Tasks expects the body as base64-encoded bytes
	body := payloadBytes

	req := &taskspb.CreateTaskRequest{
		Parent: "projects/localproject/locations/us-south1/queues/gameschedule",
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        "http://host.docker.internal:8080",
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       body,
				},
			},
		},
	}

	resp, err := client.CreateTask(ctx, req)
	if err != nil {
		log.Fatalf("CreateTask failed: %v", err)
	}

	fmt.Printf("Task created: %v\n", resp.GetName())
}
