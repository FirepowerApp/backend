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
	SendNotification(ctx context.Context, message string) (<-chan NotificationResult, error)
	GetRequiredDataKeys() []string
	FormatMessage(req NotificationRequest) string
	Close() error
}

type NotifierConfig struct {
	Config map[string]string
}
