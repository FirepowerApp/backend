package handlers

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
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

// Notifier defines the interface for sending notifications
type Notifier interface {
	// SendNotification sends a single notification and returns immediately with any initialization errors
	// The actual delivery confirmation is provided asynchronously via the returned channel
	SendNotification(ctx context.Context, req NotificationRequest) (<-chan NotificationResult, error)

	// Close cleanly shuts down the notifier and releases any resources
	Close() error
}

// DiscordNotifier implements the Notifier interface for Discord notifications
type DiscordNotifier struct {
	session   *discordgo.Session
	channelID string
	token     string
}

// NewDiscordNotifier creates a new Discord notifier instance
func NewDiscordNotifier() (*DiscordNotifier, error) {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Printf("DISCORD_BOT_TOKEN not found in environment, notifications will be skipped")
		return nil, nil
	}

	// Hardcoded channel ID as per requirements
	channelID := "1421093651202703420" // Replace with your actual Discord channel ID

	// Create Discord session
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
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
				result.Error = err
				result.Success = false
				resultChan <- result
				return
			}
		}

		// Format the message
		message := d.formatMessage(req)

		// Send the message
		log.Printf("Sending Discord message to channel %s: %s\n", d.channelID, message)
		_, err := d.session.ChannelMessageSend(d.channelID, message)
		if err != nil {
			result.Error = err
			result.Success = false
			log.Printf("Discord notification failed: %v", err)
		} else {
			result.Success = true
			log.Printf("Discord notification sent successfully: %s", notificationID)
		}

		resultChan <- result
	}()

	return resultChan, nil
}

// formatMessage creates a formatted Discord message from the notification request
func (d *DiscordNotifier) formatMessage(req NotificationRequest) string {
	message := ""

	// Extract scores and expected goals for better formatting
	homeGoals, hasHomeGoals := req.Data["homeTeamGoals"]
	awayGoals, hasAwayGoals := req.Data["awayTeamGoals"]
	homeXG, hasHomeXG := req.Data["homeTeamExpectedGoals"]
	awayXG, hasAwayXG := req.Data["awayTeamExpectedGoals"]

	// Show current score if available
	if hasHomeGoals && hasAwayGoals {
		message += "🏒 Current Score: " + req.Team1ID + " " + strconv.FormatInt(int64(homeGoals), 10) + " - " + strconv.FormatInt(int64(awayGoals), 10) + " " + req.Team2ID + "\n\n"
	}

	// Show expected goals if available
	if hasHomeXG || hasAwayXG {
		message += "📊 Expected Goals:\n"
		if hasHomeXG {
			message += "• " + req.Team1ID + ": " + strconv.FormatFloat(homeXG, 'f', 3, 64) + "\n"
		}
		if hasAwayXG {
			message += "• " + req.Team2ID + ": " + strconv.FormatFloat(awayXG, 'f', 3, 64) + "\n"
		}
	}

	message += "\n*Notification sent at " + time.Now().Format("15:04:05 MST") + "*"
	return message
}

// formatKey formats a key to be more readable
func formatKey(key string) string {
	switch key {
	case "homeTeamExpectedGoals":
		return "Home Team Expected Goals"
	case "awayTeamExpectedGoals":
		return "Away Team Expected Goals"
	case "homeTeamGoals":
		return "Home Team Goals"
	case "awayTeamGoals":
		return "Away Team Goals"
	default:
		return key
	}
}

// Close cleanly shuts down the Discord notifier
func (d *DiscordNotifier) Close() error {
	if d.session != nil {
		return d.session.Close()
	}
	return nil
}

// sendExpectedGoalsNotification uses the provided notifier to send expected goals notifications
func sendExpectedGoalsNotification(notifier Notifier, homeTeam, awayTeam, homeExpectedGoals, awayExpectedGoals, homeGoals, awayGoals string) {
	if notifier == nil {
		log.Printf("No notifier provided, skipping notification")
		return
	}

	// Parse expected goals and current score values
	data := make(map[string]float64)

	if homeExpectedGoals != "" {
		if homeVal, err := strconv.ParseFloat(homeExpectedGoals, 64); err == nil {
			data["homeTeamExpectedGoals"] = homeVal
		}
	}

	if awayExpectedGoals != "" {
		if awayVal, err := strconv.ParseFloat(awayExpectedGoals, 64); err == nil {
			data["awayTeamExpectedGoals"] = awayVal
		}
	}

	// Add current scores if available
	if homeGoals != "" {
		if homeGoalsVal, err := strconv.ParseFloat(homeGoals, 64); err == nil {
			data["homeTeamGoals"] = homeGoalsVal
		}
	}

	if awayGoals != "" {
		if awayGoalsVal, err := strconv.ParseFloat(awayGoals, 64); err == nil {
			data["awayTeamGoals"] = awayGoalsVal
		}
	}

	if len(data) == 0 {
		log.Printf("No valid data to send notification for")
		return
	}

	req := NotificationRequest{
		Team1ID: homeTeam,
		Team2ID: awayTeam,
		Data:    data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resultChan, err := notifier.SendNotification(ctx, req)
	if err != nil {
		log.Printf("Failed to send expected goals notification: %v", err)
		return
	}

	// Handle result asynchronously to avoid blocking
	go func() {
		select {
		case result := <-resultChan:
			if !result.Success {
				log.Printf("Expected goals notification failed: %v", result.Error)
			}
		case <-ctx.Done():
			log.Printf("Expected goals notification timeout")
		}
	}()
}
