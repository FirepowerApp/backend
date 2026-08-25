package season

import "testing"

func TestResolveDataSource(t *testing.T) {
	tests := []struct {
		name      string
		appEnv    string
		offseason bool
		want      DataSource
	}{
		{"staging + offseason -> emulator", "staging", true, DataSourceEmulator},
		{"staging + in-season -> live", "staging", false, DataSourceLive},
		{"production + offseason -> live (staging gate)", "production", true, DataSourceLive},
		{"non-staging env + offseason -> live", "development", true, DataSourceLive},
		{"empty env + offseason -> live", "", true, DataSourceLive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDataSource(tt.appEnv, tt.offseason)
			if got != tt.want {
				t.Errorf("ResolveDataSource(%q, %v) = %q, want %q", tt.appEnv, tt.offseason, got, tt.want)
			}
		})
	}
}

func TestResolveBaseURL(t *testing.T) {
	urls := BaseURLs{Live: "https://live.example", Emulator: "http://emulator.example"}

	tests := []struct {
		name   string
		ds     DataSource
		appEnv string
		want   string
	}{
		{"emulator + staging -> emulator URL", DataSourceEmulator, "staging", "http://emulator.example"},
		{"emulator + production (regate) -> live URL", DataSourceEmulator, "production", "https://live.example"},
		{"emulator + non-staging (regate) -> live URL", DataSourceEmulator, "development", "https://live.example"},
		{"live + staging -> live URL", DataSourceLive, "staging", "https://live.example"},
		{"live + production -> live URL", DataSourceLive, "production", "https://live.example"},
		{"empty (legacy task) -> live URL", "", "staging", "https://live.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBaseURL(tt.ds, tt.appEnv, urls)
			if got != tt.want {
				t.Errorf("ResolveBaseURL(%q, %q) = %q, want %q", tt.ds, tt.appEnv, got, tt.want)
			}
		})
	}
}
