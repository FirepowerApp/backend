// config/config.go
package config

import (
	"os"
)

type Config struct {
	Env            string
	RedisAddress   string
	RedisPassword  string
	HandlerAddress string // For logging/monitoring
}

func LoadConfig() *Config {
	redisAddr := os.Getenv("REDIS_ADDRESS")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default for local dev
	}

	return &Config{
		Env:            os.Getenv("APP_ENV"),
		RedisAddress:   redisAddr,
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		HandlerAddress: os.Getenv("HANDLER_HOST"),
	}
}
