// Package notifiers is the production entry point for building a notification.Service.
// It reads the NOTIFIERS environment variable and registers the corresponding notifiers.
// Keeping this wiring here (rather than in notification.Service) avoids the import cycle
// that would arise if notification imported notification/liveactivity.
package notifiers

import (
	"log"
	"os"
	"strings"

	"watchgameupdates/internal/notification"
	"watchgameupdates/internal/notification/liveactivity"
)

// New returns a notification.Service populated according to the NOTIFIERS env var.
// The shouldNotify flag is forwarded to the service; callers that want to suppress
// all notifications (e.g. when ShouldNotify=false on a task payload) pass false.
func New(shouldNotify bool) *notification.Service {
	svc := notification.NewServiceWithNotificationFlag(shouldNotify)

	raw := os.Getenv("NOTIFIERS")
	if raw == "" {
		log.Printf("NOTIFIERS not set; no notifiers will be registered")
		return svc
	}

	for _, entry := range strings.Split(raw, ",") {
		name := strings.TrimSpace(strings.ToLower(entry))
		switch name {
		case "discord":
			if n := tryDiscord(); n != nil {
				svc.RegisterNotifier(n)
			}
		case "liveactivity":
			if n := tryLiveActivity(); n != nil {
				svc.RegisterNotifier(n)
			}
		default:
			log.Printf("Unknown notifier %q in NOTIFIERS; skipping", name)
		}
	}

	return svc
}

func tryDiscord() notification.Notifier {
	cfg, err := notification.LoadDiscordConfigFromEnv()
	if err != nil {
		log.Printf("Discord notifier config not found or invalid: %v", err)
		return nil
	}
	n, err := notification.NewDiscordNotifier(cfg)
	if err != nil {
		log.Printf("Failed to create Discord notifier: %v", err)
		return nil
	}
	log.Printf("Discord notifier registered")
	return n
}

func tryLiveActivity() notification.Notifier {
	n, err := liveactivity.New()
	if err != nil {
		log.Printf("LiveActivity notifier not configured: %v", err)
		return nil
	}
	log.Printf("LiveActivity notifier registered")
	return n
}
