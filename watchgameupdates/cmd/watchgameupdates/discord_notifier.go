package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

// DiscordNotifier implements the Notifier interface for Discord notifications
type DiscordNotifier struct {
	session   *discordgo.Session
	channelID string
	token     string
}

// NewDiscordNotifier creates a new Discord notifier instance
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

	return &DiscordNotifier{
		session:   session,
		channelID: channelID,
		token:     token,
	}, nil
}

// SendNotification sends a single notification to Discord
func (d *DiscordNotifier) SendNotification(ctx context.Context, req NotificationRequest) (<-chan NotificationResult, error) {
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

		// Format the message
		message := d.formatMessage(req)

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

// SendBatch sends multiple notifications as a batch (processes them sequentially)
func (d *DiscordNotifier) SendBatch(ctx context.Context, batch NotificationBatch) (<-chan NotificationResult, error) {
	resultChan := make(chan NotificationResult, len(batch.Requests))

	go func() {
		defer close(resultChan)

		// Open connection once for the batch
		if d.session.State == nil {
			if err := d.session.Open(); err != nil {
				// Send error result for all requests in the batch
				for range batch.Requests {
					resultChan <- NotificationResult{
						ID:        uuid.New().String(),
						Success:   false,
						Error:     fmt.Errorf("failed to open Discord connection: %w", err),
						Timestamp: time.Now(),
					}
				}
				return
			}
		}

		// Process each notification in the batch
		for _, req := range batch.Requests {
			select {
			case <-ctx.Done():
				// Context cancelled, stop processing
				resultChan <- NotificationResult{
					ID:        uuid.New().String(),
					Success:   false,
					Error:     ctx.Err(),
					Timestamp: time.Now(),
				}
				return
			default:
				notificationID := uuid.New().String()
				result := NotificationResult{
					ID:        notificationID,
					Timestamp: time.Now(),
				}

				// Format and send the message
				message := d.formatMessage(req)
				_, err := d.session.ChannelMessageSend(d.channelID, message)
				if err != nil {
					result.Error = fmt.Errorf("failed to send Discord message: %w", err)
					result.Success = false
				} else {
					result.Success = true
					log.Printf("Discord batch notification sent successfully: %s", notificationID)
				}

				resultChan <- result

				// Small delay between messages to avoid rate limiting
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	return resultChan, nil
}

// formatMessage creates a formatted Discord message from the notification request
func (d *DiscordNotifier) formatMessage(req NotificationRequest) string {
	var builder strings.Builder

	// Header with team information
	builder.WriteString(fmt.Sprintf("🏒 **Game Update: %s vs %s**\n\n", req.Team1ID, req.Team2ID))

	// Add data dictionary information
	if len(req.Data) > 0 {
		builder.WriteString("📊 Game Statistics:\n")
		for key, value := range req.Data {
			// Format the key to be more readable (replace underscores/dashes with spaces, capitalize)
			displayKey := strings.ReplaceAll(strings.ReplaceAll(key, "_", " "), "-", " ")
			displayKey = strings.Title(strings.ToLower(displayKey))

			// Format the value appropriately
			if value == float64(int64(value)) {
				// Display as integer if it's a whole number
				builder.WriteString(fmt.Sprintf("• %s: %d\n", displayKey, int64(value)))
			} else {
				// Display with 2 decimal places for non-whole numbers
				builder.WriteString(fmt.Sprintf("• %s: %.2f\n", displayKey, value))
			}
		}
	}

	builder.WriteString(fmt.Sprintf("\n*Notification sent at %s*", time.Now().Format("15:04:05 MST")))

	return builder.String()
}

// Close cleanly shuts down the Discord notifier
func (d *DiscordNotifier) Close() error {
	if d.session != nil {
		return d.session.Close()
	}
	return nil
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
