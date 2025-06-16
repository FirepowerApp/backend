// config/config.go
package config

import (
	"os"
)

type Config struct {
	Env               string
	ProjectID         string
	QueueID           string
	LocationID        string
	UseEmulator       bool
	CloudTasksAddress string
	HandlerAddress    string
}

func LoadConfig() *Config {
	return &Config{
		Env:               os.Getenv("APP_ENV"),
		ProjectID:         os.Getenv("GCP_PROJECT_ID"),
		QueueID:           os.Getenv("CLOUD_TASKS_QUEUE"),
		LocationID:        os.Getenv("GCP_LOCATION"),
		UseEmulator:       os.Getenv("USE_TASKS_EMULATOR") == "true",
		CloudTasksAddress: os.Getenv("CLOUD_TASKS_EMULATOR_HOST"), // only in dev
		HandlerAddress:    os.Getenv("HANDLER_HOST"),
	}
}
