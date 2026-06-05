package liveactivity

// LiveActivityNotifier sends APNs broadcast push to Live Activity team channels.
//
// Dispatch flow (called by notification.Service in a goroutine per notifier):
//
//   FormatMessage(req) → JSON dispatch envelope {channels, payload}
//   SendNotification(ctx, envelope) → parses channels + payload
//       ├── Push to nhl-team-HOME channel (goroutine)
//       └── Push to nhl-team-AWAY channel (goroutine)

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	. "watchgameupdates/internal/notification"
)

const (
	maxRetries = 3
	retryDelay = time.Second
)

var requiredDataKeys = []string{
	"homeTeamGoals",
	"awayTeamGoals",
	"homeTeamExpectedGoals",
	"awayTeamExpectedGoals",
	"gameState",
	"homeTeamAbbrev",
	"awayTeamAbbrev",
	"lastPlayType",
}

// LiveActivityNotifier implements notification.Notifier.
type LiveActivityNotifier struct {
	client         *apnsClient
	useDevChannels bool
}

// New creates a LiveActivityNotifier from environment config.
// Returns an error if the feature is disabled or any required env var is missing.
func New() (*LiveActivityNotifier, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	client, err := newAPNsClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create APNs client: %w", err)
	}

	log.Printf("LiveActivity notifier initialized: host=%s topic=%s channels=%s", cfg.Host, cfg.Topic, channelEnvName(cfg.UseDevChannels))
	return &LiveActivityNotifier{client: client, useDevChannels: cfg.UseDevChannels}, nil
}

func (n *LiveActivityNotifier) GetRequiredDataKeys() []string {
	return requiredDataKeys
}

// FormatMessage builds the dispatch envelope (channels + APNs payload) as JSON.
func (n *LiveActivityNotifier) FormatMessage(req NotificationRequest) string {
	msg, err := BuildDispatchMessage(req, n.useDevChannels)
	if err != nil {
		log.Printf("ERROR: LiveActivity FormatMessage: %v", err)
		return ""
	}
	return msg
}

// SendNotification parses the dispatch envelope from FormatMessage and pushes to all channels.
func (n *LiveActivityNotifier) SendNotification(ctx context.Context, message string) (<-chan NotificationResult, error) {
	resultChan := make(chan NotificationResult, 1)

	if message == "" {
		close(resultChan)
		return resultChan, fmt.Errorf("empty dispatch message")
	}

	var env dispatchEnvelope
	if err := json.Unmarshal([]byte(message), &env); err != nil {
		close(resultChan)
		return resultChan, fmt.Errorf("parse dispatch envelope: %w", err)
	}
	if len(env.Channels) == 0 {
		close(resultChan)
		return resultChan, fmt.Errorf("dispatch envelope has no channels")
	}

	payload := []byte(env.Payload)
	channels := env.Channels
	id := uuid.New().String()

	go func() {
		defer close(resultChan)
		err := n.pushToAll(ctx, channels, payload)
		resultChan <- NotificationResult{
			ID:        id,
			Success:   err == nil,
			Error:     err,
			Timestamp: time.Now(),
		}
	}()

	return resultChan, nil
}

// pushToAll pushes payload to all channels in parallel; returns first error encountered.
func (n *LiveActivityNotifier) pushToAll(ctx context.Context, channels []string, payload []byte) error {
	errs := make(chan error, len(channels))
	for _, ch := range channels {
		go func(ch string) {
			errs <- n.pushWithRetry(ctx, ch, payload)
		}(ch)
	}

	var firstErr error
	for range channels {
		if err := <-errs; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (n *LiveActivityNotifier) pushWithRetry(ctx context.Context, channelToken string, payload []byte) error {
	delay := retryDelay
	var lastErr error

	for attempt := range maxRetries {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay *= 2
			}
		}

		err := n.client.Push(ctx, channelToken, payload)
		if err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		lastErr = err
		log.Printf("WARN: APNs retryable error ch=%s attempt=%d/%d: %v", channelToken, attempt+1, maxRetries, err)
	}
	return fmt.Errorf("APNs push failed after %d attempts: %w", maxRetries, lastErr)
}

func (n *LiveActivityNotifier) Close() error {
	return nil
}
