// internal/tasks/factory.go
package tasks

import (
	"context"
	"log"
	"watchgameupdates/config"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

func NewCloudTasksClient(ctx context.Context, cfg *config.Config) (CloudTasksClient, error) {
	if cfg.Env == "local" && cfg.CloudTasksAddress != "" {
		log.Printf("Using local Cloud Tasks emulator at %s", cfg.CloudTasksAddress)
		// Connect to emulator using plaintext (no TLS)
		conn, err := grpc.Dial(
			cfg.CloudTasksAddress,
			grpc.WithInsecure(),
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
