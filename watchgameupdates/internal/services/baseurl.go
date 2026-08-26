package services

import (
	"log"
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
	url := season.ResolveBaseURL(season.DataSource(dataSource), os.Getenv("APP_ENV"), season.BaseURLs{
		Live:     envOrDefault("PLAYBYPLAY_API_BASE_URL", defaultPlayByPlayBaseURL),
		Emulator: os.Getenv("EMULATOR_PLAYBYPLAY_BASE_URL"),
	})
	warnIfEmulatorURLMissing(dataSource, "EMULATOR_PLAYBYPLAY_BASE_URL", url)
	return url
}

func resolveStatsBaseURL(dataSource string) string {
	url := season.ResolveBaseURL(season.DataSource(dataSource), os.Getenv("APP_ENV"), season.BaseURLs{
		Live:     envOrDefault("STATS_API_BASE_URL", defaultStatsBaseURL),
		Emulator: os.Getenv("EMULATOR_STATS_BASE_URL"),
	})
	warnIfEmulatorURLMissing(dataSource, "EMULATOR_STATS_BASE_URL", url)
	return url
}

// warnIfEmulatorURLMissing logs a clear diagnostic when emulator mode was
// requested but resolved to an empty base URL (e.g. APP_ENV=staging exported
// without the rest of the emulator ConfigMap). Without this, the caller gets
// an opaque "unsupported protocol scheme" error from a malformed relative
// URL, with no indication the real cause is a missing env var.
func warnIfEmulatorURLMissing(dataSource, envKey, resolvedURL string) {
	if dataSource == string(season.DataSourceEmulator) && resolvedURL == "" {
		log.Printf("season: DataSource=emulator but %s is unset — requests will fail against an empty base URL", envKey)
	}
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
