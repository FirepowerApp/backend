package liveactivity

import (
	"fmt"
	"os"
	"strings"
)

// Config holds APNs credentials for Live Activity broadcast push.
type Config struct {
	TeamID  string // 10-char Apple Developer Team ID
	KeyID   string // .p8 key ID from App Store Connect
	AuthKey string // contents of .p8 file (not a path, container-friendly)
	Topic   string // bundle ID, e.g. me.blakenelson.firepower
	Host    string // api.push.apple.com or api.sandbox.push.apple.com
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

	host := os.Getenv("APNS_HOST")
	if host == "" {
		host = "api.push.apple.com"
	}

	return &Config{
		TeamID:  vars["APNS_TEAM_ID"],
		KeyID:   vars["APNS_KEY_ID"],
		AuthKey: vars["APNS_AUTH_KEY"],
		Topic:   vars["APNS_TOPIC"],
		Host:    host,
	}, nil
}
