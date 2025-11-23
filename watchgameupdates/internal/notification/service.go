package notification

import (
	"context"
	"log"
	"time"
)

type Service struct {
	notifiers           []Notifier
	config              ServiceConfig
	allRequiredDataKeys []string
}

type ServiceConfig struct {
	RecomputeTypes map[string]struct{}
}

func NewService() *Service {
	service := &Service{
		notifiers: []Notifier{},
		config: ServiceConfig{
			RecomputeTypes: map[string]struct{}{
				"blocked-shot": {},
				"missed-shot":  {},
				"shot-on-goal": {},
				"goal":         {},
			},
		},
	}

	service.discoverNotifiers()
	return service
}

func NewServiceWithConfig(config ServiceConfig) *Service {
	service := &Service{
		notifiers: []Notifier{},
		config:    config,
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

func (s *Service) ShouldRecomputeExpectedGoals(eventType string) bool {
	_, exists := s.config.RecomputeTypes[eventType]
	return exists
}

func (s *Service) SendGameUpdate(homeTeam, awayTeam, homeXG, awayXG, homeGoals, awayGoals string) {
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

	resultChan, err := notifier.SendNotification(ctx, req)
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
