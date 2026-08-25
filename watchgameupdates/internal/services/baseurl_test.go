package services

import "testing"

func TestResolvePlayByPlayBaseURL(t *testing.T) {
	tests := []struct {
		name       string
		dataSource string
		appEnv     string
		live       string
		emulator   string
		want       string
	}{
		{"emulator + staging -> emulator URL", "emulator", "staging", "http://live.example", "http://emu.example", "http://emu.example"},
		{"emulator + production (regate) -> live URL", "emulator", "production", "http://live.example", "http://emu.example", "http://live.example"},
		{"live -> live URL", "live", "staging", "http://live.example", "http://emu.example", "http://live.example"},
		{"empty (legacy task) -> live URL", "", "staging", "http://live.example", "http://emu.example", "http://live.example"},
		{"no live env set -> hardcoded default", "live", "staging", "", "", defaultPlayByPlayBaseURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tt.appEnv)
			t.Setenv("PLAYBYPLAY_API_BASE_URL", tt.live)
			t.Setenv("EMULATOR_PLAYBYPLAY_BASE_URL", tt.emulator)

			got := resolvePlayByPlayBaseURL(tt.dataSource)
			if got != tt.want {
				t.Errorf("resolvePlayByPlayBaseURL(%q) = %q, want %q", tt.dataSource, got, tt.want)
			}
		})
	}
}

func TestResolveStatsBaseURL(t *testing.T) {
	tests := []struct {
		name       string
		dataSource string
		appEnv     string
		live       string
		emulator   string
		want       string
	}{
		{"emulator + staging -> emulator URL", "emulator", "staging", "http://live.example", "http://emu.example", "http://emu.example"},
		{"emulator + production (regate) -> live URL", "emulator", "production", "http://live.example", "http://emu.example", "http://live.example"},
		{"live -> live URL", "live", "staging", "http://live.example", "http://emu.example", "http://live.example"},
		{"empty (legacy task) -> live URL", "", "staging", "http://live.example", "http://emu.example", "http://live.example"},
		{"no live env set -> hardcoded default", "live", "staging", "", "", defaultStatsBaseURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tt.appEnv)
			t.Setenv("STATS_API_BASE_URL", tt.live)
			t.Setenv("EMULATOR_STATS_BASE_URL", tt.emulator)

			got := resolveStatsBaseURL(tt.dataSource)
			if got != tt.want {
				t.Errorf("resolveStatsBaseURL(%q) = %q, want %q", tt.dataSource, got, tt.want)
			}
		})
	}
}
