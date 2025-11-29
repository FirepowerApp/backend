package notification

import (
	"context"
	"log"
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
		log.Printf("Discord notifier config not found or invalid: %v", err)
		return nil
	}

	notifier, err := NewDiscordNotifier(config)
	if err != nil {
		log.Printf("Failed to create Discord notifier: %v", err)
		return nil
	}

	log.Printf("Discord notifier created successfully")
	return notifier
}

func (s *Service) SendGameEventNotifications(game Game, gameData map[string]string) {
	if !s.shouldNotify {
		log.Printf("Notifications disabled for this service instance, skipping game event notifications")
		return
	}

	for i, notifier := range s.notifiers {
		data := map[string]string{}
		for _, key := range notifier.GetRequiredDataKeys() {
			if val, ok := gameData[key]; ok {
				data[key] = val
			} else {
				log.Printf("WARNING: Required data key '%s' not found in game data for notifier %d", key, i)
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
		log.Printf("Notifications disabled for this service instance, skipping game update notifications")
		return
	}

	if len(s.notifiers) == 0 {
		log.Printf("No notifiers configured, skipping notification")
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
		log.Printf("Notifier %d failed to send notification: %v", index, err)
		return
	}

	select {
	case result := <-resultChan:
		if !result.Success {
			log.Printf("Notifier %d notification failed: %v", index, result.Error)
		} else {
			log.Printf("Notifier %d notification sent successfully: %s", index, result.ID)
		}
	case <-ctx.Done():
		log.Printf("Notifier %d notification timed out", index)
	}
}

// Gracefully shuts down the service
func (s *Service) Close() error {
	var lastErr error
	for _, notifier := range s.notifiers {
		if err := notifier.Close(); err != nil {
			log.Printf("Error closing notifier: %v", err)
			lastErr = err
		}
	}
	return lastErr
}
