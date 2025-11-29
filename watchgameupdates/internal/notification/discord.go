package notification

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

type DiscordNotifier struct {
	session          *discordgo.Session
	channelID        string
	token            string
	requiredDataKeys []string
}

func NewDiscordNotifier(config NotifierConfig) (*DiscordNotifier, error) {
	token, exists := config.Config["DISCORD_BOT_TOKEN"]
	if !exists || token == "" {
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN not found in config")
	}

	// Hardcoded channel ID as per requirements
	channelID := "1421093651202703420"

	// Create Discord session
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	requiredDataKeys := []string{
		"homeTeamExpectedGoals",
		"awayTeamExpectedGoals",
		"homeTeamGoals",
		"awayTeamGoals",
		"homeTeamShootOutGoals",
		"awayTeamShootOutGoals",
	}

	return &DiscordNotifier{
		session:          session,
		channelID:        channelID,
		token:            token,
		requiredDataKeys: requiredDataKeys,
	}, nil
}

func (d *DiscordNotifier) GetRequiredDataKeys() []string {
	return d.requiredDataKeys
}

// SendNotification sends a single notification to Discord
func (d *DiscordNotifier) SendNotification(ctx context.Context, message string) (<-chan NotificationResult, error) {
	resultChan := make(chan NotificationResult, 1)
	notificationID := uuid.New().String()

	// Start goroutine to handle async notification sending
	go func() {
		defer close(resultChan)

		result := NotificationResult{
			ID:        notificationID,
			Timestamp: time.Now(),
		}

		// Open connection if not already open
		if d.session.State == nil {
			if err := d.session.Open(); err != nil {
				result.Error = fmt.Errorf("failed to open Discord connection: %w", err)
				result.Success = false
				resultChan <- result
				return
			}
		}

		// Log message content
		log.Printf("Sending Discord message: %s", message)

		// Send the message
		_, err := d.session.ChannelMessageSend(d.channelID, message)
		if err != nil {
			result.Error = fmt.Errorf("failed to send Discord message: %w", err)
			result.Success = false
		} else {
			result.Success = true
			log.Printf("Discord notification sent successfully: %s", notificationID)
		}

		resultChan <- result
	}()

	return resultChan, nil
}

// Close cleanly shuts down the Discord notifier
func (d *DiscordNotifier) Close() error {
	if d.session != nil {
		return d.session.Close()
	}
	return nil
}

// formatMessage creates a formatted Discord message from the notification request
func (d *DiscordNotifier) FormatMessage(req NotificationRequest) string {
	message := ""

	homeGoals, hasHomeGoals := req.Data["homeTeamGoals"]
	awayGoals, hasAwayGoals := req.Data["awayTeamGoals"]
	homeXG, hasHomeXG := req.Data["homeTeamExpectedGoals"]
	awayXG, hasAwayXG := req.Data["awayTeamExpectedGoals"]

	if hasHomeGoals && hasAwayGoals {
		message += "🏒 Current Score: " + req.Team1ID + " " + homeGoals + " - " + awayGoals + " " + req.Team2ID + "\n\n"
	}

	// Show expected goals if available
	if hasHomeXG || hasAwayXG {
		message += "📊 Expected Goals:\n"
		if hasHomeXG {
			message += "• " + req.Team1ID + ": " + homeXG + "\n"
		}
		if hasAwayXG {
			message += "• " + req.Team2ID + ": " + awayXG + "\n"
		}
	}

	message += "\n*Notification sent at " + time.Now().Format("15:04:05 MST") + "*"
	return message
}

// LoadDiscordConfigFromEnv loads Discord configuration from environment variables
func LoadDiscordConfigFromEnv() (NotifierConfig, error) {
	config := NotifierConfig{
		Config: make(map[string]string),
	}

	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		return config, fmt.Errorf("DISCORD_BOT_TOKEN environment variable is required")
	}

	config.Config["DISCORD_BOT_TOKEN"] = token
	return config, nil
}
