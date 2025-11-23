package notification

import (
	"context"
	"time"
)

type NotificationResult struct {
	ID        string
	Success   bool
	Error     error
	Timestamp time.Time
}

type NotificationRequest struct {
	Team1ID string
	Team2ID string
	Data    map[string]string
}

type NotificationBatch struct {
	Requests []NotificationRequest
}

type Notifier interface {
	SendNotification(ctx context.Context, req NotificationRequest) (<-chan NotificationResult, error)
	SendBatch(ctx context.Context, batch NotificationBatch) (<-chan NotificationResult, error)
	GetRequiredDataKeys() []string
	Close() error
}

type NotifierConfig struct {
	Config map[string]string
}
