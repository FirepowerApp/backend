package liveactivity

import "testing"

// allTricodes is the full NHL roster the backend must be able to notify. It
// mirrors the 32-team set in the iOS source of truth
// (github.com/FirepowerApp/ios, main:Firepower/NHLTeams.swift). Keeping it
// explicit here means a team can never silently drop out of the prod map
// without this test failing.
var allTricodes = []string{
	"ANA", "BOS", "BUF", "CAR", "CBJ", "CGY", "CHI", "COL",
	"DAL", "DET", "EDM", "FLA", "LAK", "MIN", "MTL", "NJD",
	"NSH", "NYI", "NYR", "OTT", "PHI", "PIT", "SEA", "SJS",
	"STL", "TBL", "TOR", "UTA", "VAN", "VGK", "WPG", "WSH",
}

// TestReplicateAll32ProdChannels enforces the core invariant of this design:
// every one of the 32 teams resolves to a non-empty production channel ID, so
// channelForTeam never fails for a missing ID. This makes the scheduler's
// TEAM_FILTER the sole guard on which teams send notifications. If iOS mints a
// channel for a new/blank team, copy the ID into prodChannels or this fails.
func TestReplicateAll32ProdChannels(t *testing.T) {
	if len(prodChannels) != len(allTricodes) {
		t.Fatalf("prodChannels has %d entries, want %d", len(prodChannels), len(allTricodes))
	}
	for _, tri := range allTricodes {
		id, ok := channelForTeam(tri, false)
		if !ok || id == "" {
			t.Errorf("team %s has no production channel ID; every team must be populated so the scheduler filter is the only guard", tri)
		}
	}
}

// TestNoUnknownProdTeams guards the other direction: prodChannels must not carry
// a tricode outside the known 32-team roster (a typo would create a channel that
// no device ever subscribes to).
func TestNoUnknownProdTeams(t *testing.T) {
	known := make(map[string]bool, len(allTricodes))
	for _, tri := range allTricodes {
		known[tri] = true
	}
	for tri := range prodChannels {
		if !known[tri] {
			t.Errorf("prodChannels has unknown tricode %q", tri)
		}
	}
}
