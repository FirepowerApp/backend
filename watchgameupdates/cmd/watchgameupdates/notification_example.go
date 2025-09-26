package main

import (
	"context"
	"log"
	"time"

	"github.com/joho/godotenv"
)

// ExampleNotificationUsage demonstrates how to use the notification system
func ExampleNotificationUsage() {
	// Load environment variables (typically done in main.go)
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Load Discord configuration from environment
	config, err := LoadDiscordConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load Discord config: %v", err)
	}

	// Create Discord notifier
	notifier, err := NewDiscordNotifier(config)
	if err != nil {
		log.Fatalf("Failed to create Discord notifier: %v", err)
	}
	defer notifier.Close()

	// Example 1: Send a single notification
	ctx := context.Background()

	singleNotification := NotificationRequest{
		Team1ID: "BOS", // Boston Bruins
		Team2ID: "MTL", // Montreal Canadiens
		Data: map[string]float64{
			"goals_team1":         2,
			"goals_team2":         1,
			"shots_on_goal_team1": 15,
			"shots_on_goal_team2": 12,
			"power_play_goals":    1,
			"penalty_minutes":     6,
			"face_off_win_pct":    58.5,
			"save_percentage":     0.923,
		},
	}

	// Send the notification
	resultChan, err := notifier.SendNotification(ctx, singleNotification)
	if err != nil {
		log.Fatalf("Failed to send notification: %v", err)
	}

	// Wait for the result asynchronously
	go func() {
		result := <-resultChan
		if result.Success {
			log.Printf("Notification sent successfully! ID: %s", result.ID)
		} else {
			log.Printf("Notification failed: %v", result.Error)
		}
	}()

	// Example 2: Send a batch of notifications
	batch := NotificationBatch{
		Requests: []NotificationRequest{
			{
				Team1ID: "TOR", // Toronto Maple Leafs
				Team2ID: "OTT", // Ottawa Senators
				Data: map[string]float64{
					"goals_team1":    3,
					"goals_team2":    2,
					"period":         2,
					"time_remaining": 847, // seconds
				},
			},
			{
				Team1ID: "NYR", // New York Rangers
				Team2ID: "NYI", // New York Islanders
				Data: map[string]float64{
					"goals_team1": 1,
					"goals_team2": 0,
					"period":      1,
					"shots_team1": 8,
					"shots_team2": 5,
				},
			},
		},
	}

	// Send the batch
	batchResultChan, err := notifier.SendBatch(ctx, batch)
	if err != nil {
		log.Fatalf("Failed to send batch notifications: %v", err)
	}

	// Process batch results as they come in
	go func() {
		for result := range batchResultChan {
			if result.Success {
				log.Printf("Batch notification sent successfully! ID: %s", result.ID)
			} else {
				log.Printf("Batch notification failed: %v", result.Error)
			}
		}
		log.Println("All batch notifications processed")
	}()

	// In a real application, you would typically have a longer-running process
	// For this example, we'll just wait a bit to see the results
	time.Sleep(5 * time.Second)
}

// IntegrateWithExistingHandler shows how to integrate notifications into your existing handler
func IntegrateWithExistingHandler() {
	// This would typically be called from your existing handler function
	// when certain game events occur

	// Initialize notifier (you'd probably do this once and reuse)
	config, err := LoadDiscordConfigFromEnv()
	if err != nil {
		log.Printf("Failed to load Discord config: %v", err)
		return
	}

	notifier, err := NewDiscordNotifier(config)
	if err != nil {
		log.Printf("Failed to create Discord notifier: %v", err)
		return
	}
	defer notifier.Close()

	// Example: Trigger notification when a goal is scored
	goalNotification := NotificationRequest{
		Team1ID: "BOS",
		Team2ID: "MTL",
		Data: map[string]float64{
			"goal_scored":       1,
			"scoring_team":      1, // Team 1 scored
			"total_goals_team1": 3,
			"total_goals_team2": 1,
			"period":            2,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resultChan, err := notifier.SendNotification(ctx, goalNotification)
	if err != nil {
		log.Printf("Failed to send goal notification: %v", err)
		return
	}

	// Handle the result asynchronously so it doesn't block your main handler
	go func() {
		select {
		case result := <-resultChan:
			if result.Success {
				log.Printf("Goal notification sent successfully!")
			} else {
				log.Printf("Goal notification failed: %v", result.Error)
			}
		case <-ctx.Done():
			log.Printf("Notification timeout")
		}
	}()
}
