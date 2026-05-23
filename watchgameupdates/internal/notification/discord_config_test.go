package notification

import "testing"

func TestLoadDiscordConfigFromEnv_Success(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	t.Setenv("DISCORD_CHANNEL_ID", "123456789")

	cfg, err := LoadDiscordConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Config["DISCORD_BOT_TOKEN"] != "test-token" {
		t.Errorf("expected DISCORD_BOT_TOKEN in config")
	}
	if cfg.Config["DISCORD_CHANNEL_ID"] != "123456789" {
		t.Errorf("expected DISCORD_CHANNEL_ID in config")
	}
}

func TestLoadDiscordConfigFromEnv_MissingToken(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "")
	t.Setenv("DISCORD_CHANNEL_ID", "123456789")

	_, err := LoadDiscordConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for missing DISCORD_BOT_TOKEN, got nil")
	}
}

func TestLoadDiscordConfigFromEnv_MissingChannelID(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	t.Setenv("DISCORD_CHANNEL_ID", "")

	_, err := LoadDiscordConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for missing DISCORD_CHANNEL_ID, got nil")
	}
}
