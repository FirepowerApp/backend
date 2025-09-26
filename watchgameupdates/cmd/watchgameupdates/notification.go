package main

import (
	"context"
	"time"
)

// NotificationResult represents the result of a notification attempt
type NotificationResult struct {
	ID        string    // Unique identifier for this notification
	Success   bool      // Whether the notification was sent successfully
	Error     error     // Error if the notification failed
	Timestamp time.Time // When the notification was processed
}

// NotificationRequest represents a single notification to be sent
type NotificationRequest struct {
	Team1ID string             // ID of the first team
	Team2ID string             // ID of the second team
	Data    map[string]float64 // Key-value pairs of data to include in the notification
}

// NotificationBatch represents a batch of notifications to be sent
type NotificationBatch struct {
	Requests []NotificationRequest
}

// Notifier defines the interface for sending notifications
type Notifier interface {
	// SendNotification sends a single notification and returns immediately with any initialization errors
	// The actual delivery confirmation is provided asynchronously via the returned channel
	SendNotification(ctx context.Context, req NotificationRequest) (<-chan NotificationResult, error)

	// SendBatch sends multiple notifications as a batch operation
	// Returns a channel that will provide results for each notification in the batch
	SendBatch(ctx context.Context, batch NotificationBatch) (<-chan NotificationResult, error)

	// Close cleanly shuts down the notifier and releases any resources
	Close() error
}

// NotifierConfig holds configuration for notification implementations
type NotifierConfig struct {
	// Platform-specific configuration can be added here
	// For Discord: token, channel ID, etc.
	// For APNS: certificates, bundle ID, etc.
	Config map[string]string
}
