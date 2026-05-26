package notification

import (
	"context"
	"log/slog"
	"sync"
	"time"

	. "watchgameupdates/internal/models"
)

type Service struct {
	notifiers           []Notifier
	allRequiredDataKeys []string
	shouldNotify        bool
}

func NewService() *Service {
	return NewServiceWithNotificationFlag(true) // Default to true for backward compatibility
}

func NewServiceWithNotificationFlag(shouldNotify bool) *Service {
	service := &Service{
		notifiers:    []Notifier{},
		shouldNotify: shouldNotify,
	}

	service.discoverNotifiers()
	return service
}

func (s *Service) GetAllRequiredDataKeys() []string {
	return s.allRequiredDataKeys
}

func (s *Service) discoverNotifiers() {
	if discordNotifier := s.tryCreateDiscordNotifier(); discordNotifier != nil {
		s.allRequiredDataKeys = append(s.allRequiredDataKeys, discordNotifier.GetRequiredDataKeys()...)
		s.notifiers = append(s.notifiers, discordNotifier)
	}
}

func (s *Service) tryCreateDiscordNotifier() Notifier {
	config, err := LoadDiscordConfigFromEnv()
	if err != nil {
		slog.Debug("Discord notifier not configured", "reason", err)
		return nil
	}

	notifier, err := NewDiscordNotifier(config)
	if err != nil {
		slog.Error("failed to create Discord notifier", "error", err)
		return nil
	}

	slog.Info("Discord notifier created successfully")
	return notifier
}

func (s *Service) SendGameEventNotifications(game Game, gameData map[string]string) {
	if !s.shouldNotify {
		slog.Debug("notifications disabled, skipping game event notifications")
		return
	}

	for i, notifier := range s.notifiers {
		data := map[string]string{}
		for _, key := range notifier.GetRequiredDataKeys() {
			if val, ok := gameData[key]; ok {
				data[key] = val
			} else {
				slog.Warn("required data key missing for notifier", "key", key, "notifier_index", i)
			}
		}

		req := NotificationRequest{
			Team1ID: game.HomeTeam.CommonName["default"],
			Team2ID: game.AwayTeam.CommonName["default"],
			Data:    data,
		}

		go s.sendToNotifier(notifier, req, i)
	}
}

func (s *Service) SendGameUpdate(homeTeam, awayTeam, homeXG, awayXG, homeGoals, awayGoals string) {
	if !s.shouldNotify {
		slog.Debug("notifications disabled, skipping game update notification")
		return
	}

	if len(s.notifiers) == 0 {
		slog.Debug("no notifiers configured, skipping notification")
		return
	}

	req := NotificationRequest{
		Team1ID: homeTeam,
		Team2ID: awayTeam,
		Data: map[string]string{
			"homeExpectedGoals": homeXG,
			"awayExpectedGoals": awayXG,
			"homeGoals":         homeGoals,
			"awayGoals":         awayGoals,
		},
	}

	for i, notifier := range s.notifiers {
		go s.sendToNotifier(notifier, req, i)
	}
}

func (s *Service) sendToNotifier(notifier Notifier, req NotificationRequest, index int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	message := notifier.FormatMessage(req)
	resultChan, err := notifier.SendNotification(ctx, message)
	if err != nil {
		slog.Error("notifier failed to send notification", "notifier_index", index, "error", err)
		return
	}

	select {
	case result := <-resultChan:
		if !result.Success {
			slog.Error("notification send failed", "notifier_index", index, "error", result.Error)
		} else {
			slog.Info("notification sent successfully", "notifier_index", index, "message_id", result.ID)
		}
	case <-ctx.Done():
		slog.Warn("notification timed out", "notifier_index", index)
	}
}

// SendMessage sends a plain-text message to all configured notifiers and waits for completion.
func (s *Service) SendMessage(ctx context.Context, message string) {
	if !s.shouldNotify {
		slog.Debug("notifications disabled, skipping message")
		return
	}

	if len(s.notifiers) == 0 {
		slog.Debug("no notifiers configured, skipping notification")
		return
	}

	var wg sync.WaitGroup
	for i, notifier := range s.notifiers {
		wg.Add(1)
		go func(n Notifier, idx int) {
			defer wg.Done()
			notifCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			resultChan, err := n.SendNotification(notifCtx, message)
			if err != nil {
				slog.Error("notifier failed to send message", "notifier_index", idx, "error", err)
				return
			}

			select {
			case result := <-resultChan:
				if !result.Success {
					slog.Error("message send failed", "notifier_index", idx, "error", result.Error)
				} else {
					slog.Info("message sent successfully", "notifier_index", idx, "message_id", result.ID)
				}
			case <-notifCtx.Done():
				slog.Warn("message send timed out", "notifier_index", idx)
			}
		}(notifier, i)
	}
	wg.Wait()
}

// Gracefully shuts down the service
func (s *Service) Close() error {
	var lastErr error
	for _, notifier := range s.notifiers {
		if err := notifier.Close(); err != nil {
			slog.Error("error closing notifier", "error", err)
			lastErr = err
		}
	}
	return lastErr
}

// RegisterNotifier adds a notifier and its required data keys to the service.
// Call this after NewService to inject optional notifiers (e.g. LiveActivity).
func (s *Service) RegisterNotifier(n Notifier) {
	s.allRequiredDataKeys = append(s.allRequiredDataKeys, n.GetRequiredDataKeys()...)
	s.notifiers = append(s.notifiers, n)
}
