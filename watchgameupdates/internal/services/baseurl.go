package services

import (
	"os"
	"watchgameupdates/internal/season"
)

const (
	defaultPlayByPlayBaseURL = "https://api-web.nhle.com"
	defaultStatsBaseURL      = "https://moneypuck.com"
)

// resolvePlayByPlayBaseURL and resolveStatsBaseURL are the single shared
// mapping from a task's DataSource to a concrete base URL, used by both the
// play-by-play and stats fetchers so the enum->URL logic (including the
// APP_ENV staging re-gate) lives in exactly one place.
func resolvePlayByPlayBaseURL(dataSource string) string {
	return season.ResolveBaseURL(season.DataSource(dataSource), os.Getenv("APP_ENV"), season.BaseURLs{
		Live:     envOrDefault("PLAYBYPLAY_API_BASE_URL", defaultPlayByPlayBaseURL),
		Emulator: os.Getenv("EMULATOR_PLAYBYPLAY_BASE_URL"),
	})
}

func resolveStatsBaseURL(dataSource string) string {
	return season.ResolveBaseURL(season.DataSource(dataSource), os.Getenv("APP_ENV"), season.BaseURLs{
		Live:     envOrDefault("STATS_API_BASE_URL", defaultStatsBaseURL),
		Emulator: os.Getenv("EMULATOR_STATS_BASE_URL"),
	})
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
