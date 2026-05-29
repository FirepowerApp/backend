package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a configured *slog.Logger driven by LOG_FORMAT and LOG_LEVEL env vars.
// LOG_FORMAT: "json" (default) or "text" for human-readable structured output.
// LOG_LEVEL:  "debug", "info" (default), "warn", or "error".
// Call slog.SetDefault(logger.New()) in main so all packages using slog share the config.
func New() *slog.Logger {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
