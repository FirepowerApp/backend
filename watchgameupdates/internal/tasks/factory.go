// internal/tasks/factory.go
package tasks

import (
	"context"
	"os"
	"watchgameupdates/config"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

func NewCloudTasksClient(ctx context.Context, cfg *config.Config) (CloudTasksClient, error) {
	if cfg.Env == "local" && cfg.CloudTasksAddress != "" {
		// Connect to emulator using plaintext (no TLS)
		conn, err := grpc.Dial(
			os.Getenv("CLOUD_TASKS_EMULATOR_HOST"),
			grpc.WithInsecure(), // 👈 disables TLS
		)
		if err != nil {
			return nil, err
		}
		client, err := cloudtasks.NewClient(ctx, option.WithGRPCConn(conn))
		if err != nil {
			return nil, err
		}
		return &realClient{client: client}, nil
	}

	// Production client with default credentials
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &realClient{client: client}, nil
}
