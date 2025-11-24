// config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env                    string
	ProjectID              string
	QueueID                string
	LocationID             string
	UseEmulator            bool
	CloudTasksAddress      string
	HandlerAddress         string
	MessageIntervalSeconds int
}

// TODO: Add support for customizable time between data queries
// (to prevent long delays between events during local scenario testing)

func LoadConfig() *Config {
	return &Config{
		Env:               os.Getenv("APP_ENV"),
		ProjectID:         os.Getenv("GCP_PROJECT_ID"),
		QueueID:           os.Getenv("CLOUD_TASKS_QUEUE"),
		LocationID:        os.Getenv("GCP_LOCATION"),
		UseEmulator:       os.Getenv("USE_TASKS_EMULATOR") == "true",
		CloudTasksAddress: os.Getenv("CLOUD_TASKS_EMULATOR_HOST"), // only in dev
		HandlerAddress:    os.Getenv("HANDLER_HOST"),
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
	}
}
