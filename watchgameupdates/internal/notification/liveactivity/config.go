package liveactivity

import (
	"fmt"
	"os"
	"strings"
)

// Config holds APNs credentials for Live Activity broadcast push.
type Config struct {
	TeamID         string // 10-char Apple Developer Team ID
	KeyID          string // .p8 key ID from App Store Connect
	AuthKey        string // contents of .p8 file (not a path, container-friendly)
	Topic          string // bundle ID, e.g. com.blakenelson.Firepower
	Host           string // api.push.apple.com or api.sandbox.push.apple.com
	UseDevChannels bool   // true → use debugChannels (development APNs env); set via APNS_CHANNEL_ENV=development
}

func LoadConfig() (*Config, error) {
	if !strings.EqualFold(os.Getenv("LIVEACTIVITY_PUSH_ENABLED"), "true") {
		return nil, fmt.Errorf("LIVEACTIVITY_PUSH_ENABLED is not true")
	}

	vars := map[string]string{
		"APNS_TEAM_ID":  os.Getenv("APNS_TEAM_ID"),
		"APNS_KEY_ID":   os.Getenv("APNS_KEY_ID"),
		"APNS_AUTH_KEY": os.Getenv("APNS_AUTH_KEY"),
		"APNS_TOPIC":    os.Getenv("APNS_TOPIC"),
	}
	for k, v := range vars {
		if v == "" {
			return nil, fmt.Errorf("required env var %s is not set", k)
		}
	}

	// Environment is a single concept: the broadcast channel IDs (apns-channel-id)
	// and the APNs host must belong to the same environment. If they disagree, APNs
	// rejects the push with 403 BadEnvironmentKeyInToken. APNS_CHANNEL_ENV is the
	// single source of truth; the host is derived from it. APNS_HOST may still be
	// set as an explicit override (e.g. a test proxy), but it must match the
	// channel environment or we fail fast at startup rather than at push time.
	useDevChannels := strings.EqualFold(os.Getenv("APNS_CHANNEL_ENV"), "development")
	expectedHost := "api.push.apple.com"
	if useDevChannels {
		expectedHost = "api.sandbox.push.apple.com"
	}

	host := os.Getenv("APNS_HOST")
	switch host {
	case "":
		host = expectedHost
	case expectedHost:
		// explicit override agrees with channel environment
	default:
		return nil, fmt.Errorf(
			"APNS_HOST %q does not match APNS_CHANNEL_ENV (%s channels expect host %q); "+
				"mismatched channel/host environments cause APNs 403 BadEnvironmentKeyInToken",
			host, channelEnvName(useDevChannels), expectedHost)
	}

	return &Config{
		TeamID:         vars["APNS_TEAM_ID"],
		KeyID:          vars["APNS_KEY_ID"],
		AuthKey:        vars["APNS_AUTH_KEY"],
		Topic:          vars["APNS_TOPIC"],
		Host:           host,
		UseDevChannels: useDevChannels,
	}, nil
}

func channelEnvName(useDevChannels bool) string {
	if useDevChannels {
		return "development"
	}
	return "production"
}
