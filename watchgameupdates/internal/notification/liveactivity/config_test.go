package liveactivity

import "testing"

func TestLoadConfig_Disabled(t *testing.T) {
	t.Setenv("LIVEACTIVITY_PUSH_ENABLED", "")
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when feature disabled, got nil")
	}
}

func TestLoadConfig_MissingRequiredVars(t *testing.T) {
	base := map[string]string{
		"APNS_TEAM_ID":  "TEAMID1234",
		"APNS_KEY_ID":   "KEYID12345",
		"APNS_AUTH_KEY": "dummykey",
		"APNS_TOPIC":    "me.blakenelson.firepower",
	}
	for _, missing := range []string{"APNS_TEAM_ID", "APNS_KEY_ID", "APNS_AUTH_KEY", "APNS_TOPIC"} {
		missing := missing
		t.Run(missing, func(t *testing.T) {
			t.Setenv("LIVEACTIVITY_PUSH_ENABLED", "true")
			for k, v := range base {
				t.Setenv(k, v)
			}
			t.Setenv(missing, "")
			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("expected error when %s is missing, got nil", missing)
			}
		})
	}
}

func TestLoadConfig_DefaultsHostToProduction(t *testing.T) {
	t.Setenv("LIVEACTIVITY_PUSH_ENABLED", "true")
	t.Setenv("APNS_TEAM_ID", "TEAMID1234")
	t.Setenv("APNS_KEY_ID", "KEYID12345")
	t.Setenv("APNS_AUTH_KEY", "dummykey")
	t.Setenv("APNS_TOPIC", "me.blakenelson.firepower")
	t.Setenv("APNS_HOST", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "api.push.apple.com" {
		t.Errorf("want default host api.push.apple.com, got %q", cfg.Host)
	}
}

func TestLoadConfig_SandboxHost(t *testing.T) {
	t.Setenv("LIVEACTIVITY_PUSH_ENABLED", "true")
	t.Setenv("APNS_TEAM_ID", "TEAMID1234")
	t.Setenv("APNS_KEY_ID", "KEYID12345")
	t.Setenv("APNS_AUTH_KEY", "dummykey")
	t.Setenv("APNS_TOPIC", "me.blakenelson.firepower")
	t.Setenv("APNS_HOST", "api.sandbox.push.apple.com")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "api.sandbox.push.apple.com" {
		t.Errorf("want sandbox host, got %q", cfg.Host)
	}
}
