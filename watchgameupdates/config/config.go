// config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env                      string
	MessageIntervalSeconds   int
	PeriodEndIntervalSeconds int

	// Cloud Tasks configuration (HTTP mode)
	ProjectID         string
	QueueID           string
	LocationID        string
	UseEmulator       bool
	CloudTasksAddress string
	HandlerAddress    string

	// Redis configuration (worker mode)
	RedisAddress  string
	RedisPassword string
	RedisDB       int

	// Scheduler-specific
	ScheduleAPIBaseURL   string
	ScheduleFile         string
	ScheduleDate         string
	GameMaxDurationHours int
	SchedulerNotify      bool
	TeamFilter           string
	IncludeLiveGames     bool
	SchedulerQueue       string // "cloudtasks" (default) or "redis"
}

func LoadConfig() *Config {
	return &Config{
		Env: os.Getenv("APP_ENV"),

		// Cloud Tasks (preserved for HTTP mode)
		ProjectID:         os.Getenv("GCP_PROJECT_ID"),
		QueueID:           os.Getenv("CLOUD_TASKS_QUEUE"),
		LocationID:        os.Getenv("GCP_LOCATION"),
		UseEmulator:       os.Getenv("USE_TASKS_EMULATOR") == "true",
		CloudTasksAddress: os.Getenv("CLOUD_TASKS_EMULATOR_HOST"),
		HandlerAddress:    os.Getenv("HANDLER_HOST"),

		// Redis (worker mode)
		RedisAddress:  getEnvOrDefault("REDIS_ADDRESS", "localhost:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB: func() int {
			if val, ok := os.LookupEnv("REDIS_DB"); ok {
				var intVal int
				_, err := fmt.Sscanf(val, "%d", &intVal)
				if err == nil && intVal >= 0 {
					return intVal
				}
			}
			return 0
		}(),

		MessageIntervalSeconds: func() int {
			if val, ok := os.LookupEnv("MESSAGE_INTERVAL_SECONDS"); ok {
				var intVal int
				_, err := fmt.Sscanf(val, "%d", &intVal)
				if err == nil && intVal > 0 {
					return intVal
				}
				fmt.Printf("Invalid MESSAGE_INTERVAL_SECONDS value '%s', using default of 60 seconds\n", val)
			}
			return 60 // default value
		}(),
		PeriodEndIntervalSeconds: func() int {
			if val, ok := os.LookupEnv("PERIOD_END_INTERVAL_SECONDS"); ok {
				var intVal int
				_, err := fmt.Sscanf(val, "%d", &intVal)
				if err == nil && intVal > 0 {
					return intVal
				}
				fmt.Printf("Invalid PERIOD_END_INTERVAL_SECONDS value '%s', using default of 1200 seconds\n", val)
			}
			return 1200 // default: 20 minutes
		}(),
		ScheduleAPIBaseURL: func() string {
			if val := os.Getenv("SCHEDULE_API_BASE_URL"); val != "" {
				return val
			}
			return os.Getenv("PLAYBYPLAY_API_BASE_URL")
		}(),
		ScheduleFile: os.Getenv("SCHEDULE_FILE"),
		ScheduleDate: os.Getenv("SCHEDULE_DATE"),
		TeamFilter:       os.Getenv("TEAM_FILTER"),
		IncludeLiveGames: os.Getenv("INCLUDE_LIVE_GAMES") == "true",
		SchedulerQueue:   getEnvOrDefault("SCHEDULER_QUEUE", "cloudtasks"),
		GameMaxDurationHours: func() int {
			if val, ok := os.LookupEnv("GAME_MAX_DURATION_HOURS"); ok {
				var intVal int
				_, err := fmt.Sscanf(val, "%d", &intVal)
				if err == nil && intVal > 0 {
					return intVal
				}
				fmt.Printf("Invalid GAME_MAX_DURATION_HOURS value '%s', using default of 5 hours\n", val)
			}
			return 5
		}(),
		SchedulerNotify: func() bool {
			val, ok := os.LookupEnv("SCHEDULER_SHOULD_NOTIFY")
			if !ok {
				return true // default to true
			}
			return val == "true"
		}(),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
