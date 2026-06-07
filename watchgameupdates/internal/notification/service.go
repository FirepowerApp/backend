package notification

import (
	"context"
	"log"
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
	return &Service{
		notifiers:    []Notifier{},
		shouldNotify: shouldNotify,
	}
}

func (s *Service) GetAllRequiredDataKeys() []string {
	return s.allRequiredDataKeys
}

func (s *Service) SendGameEventNotifications(game Game, gameData map[string]string) {
	if !s.shouldNotify {
		log.Printf("Notifications disabled for this service instance, skipping game event notifications")
		return
	}

	// Fields sourced from Game (not the MoneyPuck data map) — notifiers that
	// declare these in GetRequiredDataKeys can read them from a single map.
	enriched := map[string]string{
		"homeTeamAbbrev": game.HomeTeam.Abbrev,
		"awayTeamAbbrev": game.AwayTeam.Abbrev,
	}
	for k, v := range gameData {
		enriched[k] = v
	}

	for i, notifier := range s.notifiers {
		data := map[string]string{}
		for _, key := range notifier.GetRequiredDataKeys() {
			if val, ok := enriched[key]; ok {
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

// SendMessage sends a plain-text message to all configured notifiers and waits for completion.
func (s *Service) SendMessage(ctx context.Context, message string) {
	if !s.shouldNotify {
		log.Printf("Notifications disabled for this service instance, skipping message")
		return
	}

	if len(s.notifiers) == 0 {
		log.Printf("No notifiers configured, skipping notification")
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
				log.Printf("Notifier %d failed to send message: %v", idx, err)
				return
			}

			select {
			case result := <-resultChan:
				if !result.Success {
					log.Printf("Notifier %d message failed: %v", idx, result.Error)
				} else {
					log.Printf("Notifier %d message sent successfully: %s", idx, result.ID)
				}
			case <-notifCtx.Done():
				log.Printf("Notifier %d message timed out", idx)
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
			log.Printf("Error closing notifier: %v", err)
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

// WithShouldNotify returns a per-request view of this service with the given
// notification flag. Notifier instances (and their JWT/connection state) are
// shared with the original — call this on a long-lived service to avoid
// recreating notifiers on every request.
func (s *Service) WithShouldNotify(shouldNotify bool) *Service {
	return &Service{
		notifiers:           s.notifiers,
		allRequiredDataKeys: s.allRequiredDataKeys,
		shouldNotify:        shouldNotify,
	}
}
