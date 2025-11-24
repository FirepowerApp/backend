package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"watchgameupdates/config"
	"watchgameupdates/internal/models"

	"github.com/hibiken/asynq"
)

// ScheduleNextCheck schedules the next game check task in Redis
func ScheduleNextCheck(payload models.Payload) error {
	cfg := config.LoadConfig()

	// Create Asynq client
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
	})
	defer client.Close()

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create task
	task := asynq.NewTask("game:check", payloadBytes)

	// Schedule 60 seconds from now (same as Cloud Tasks implementation)
	scheduleTime := time.Now().Add(60 * time.Second)

	// Enqueue with options
	info, err := client.Enqueue(
		task,
		asynq.ProcessAt(scheduleTime),
		asynq.Queue("default"), // Can be "critical", "default", or "low"
		asynq.MaxRetry(3),      // Retry up to 3 times on failure
		asynq.Timeout(5*time.Minute), // Timeout for processing
	)

	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Printf("Successfully scheduled next check for game %s at %v (Task ID: %s)",
		payload.Game.ID,
		scheduleTime.Format(time.RFC3339),
		info.ID,
	)

	return nil
}

// ScheduleGameCheck is a convenience function to schedule a game check with optional priority
func ScheduleGameCheck(payload models.Payload, priority string, delay time.Duration) error {
	cfg := config.LoadConfig()

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
	})
	defer client.Close()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask("game:check", payloadBytes)
	scheduleTime := time.Now().Add(delay)

	// Map priority string to queue name
	queueName := "default"
	switch priority {
	case "high", "critical":
		queueName = "critical"
	case "low":
		queueName = "low"
	default:
		queueName = "default"
	}

	info, err := client.Enqueue(
		task,
		asynq.ProcessAt(scheduleTime),
		asynq.Queue(queueName),
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
	)

	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Printf("Scheduled game %s check in %v (Queue: %s, Task ID: %s)",
		payload.Game.ID,
		delay,
		queueName,
		info.ID,
	)

	return nil
}

// ScheduleImmediateCheck schedules a game check to run immediately
func ScheduleImmediateCheck(payload models.Payload) error {
	cfg := config.LoadConfig()

	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
	})
	defer client.Close()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask("game:check", payloadBytes)

	// Enqueue without delay
	info, err := client.Enqueue(
		task,
		asynq.Queue("critical"), // Immediate tasks go to critical queue
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
	)

	if err != nil {
		return fmt.Errorf("failed to enqueue immediate task: %w", err)
	}

	log.Printf("Scheduled immediate check for game %s (Task ID: %s)",
		payload.Game.ID,
		info.ID,
	)

	return nil
}

// GetQueueStats returns statistics about the task queues (useful for monitoring)
func GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
	cfg := config.LoadConfig()

	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
	})

	stats := make(map[string]interface{})

	// Get stats for each queue
	for _, queueName := range []string{"critical", "default", "low"} {
		queueStats, err := inspector.GetQueueInfo(queueName)
		if err != nil {
			log.Printf("Failed to get stats for queue %s: %v", queueName, err)
			continue
		}

		stats[queueName] = map[string]interface{}{
			"pending":    queueStats.Pending,
			"active":     queueStats.Active,
			"scheduled":  queueStats.Scheduled,
			"retry":      queueStats.Retry,
			"archived":   queueStats.Archived,
			"completed":  queueStats.Completed,
			"processed":  queueStats.Processed,
			"failed":     queueStats.Failed,
			"paused":     queueStats.Paused,
			"size":       queueStats.Size,
			"latency_ms": queueStats.Latency.Milliseconds(),
		}
	}

	return stats, nil
}
