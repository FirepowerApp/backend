package notification

import (
	"strings"
	"testing"
)

type formatMessageTestCase struct {
	name                   string
	team1ID                string
	team2ID                string
	homeGoals              string
	awayGoals              string
	homeXG                 string
	awayXG                 string
	homeShootOutGoals      string
	awayShootOutGoals      string
	expectedScore          string
	expectedHomeXG         string
	expectedAwayXG         string
	shouldContainScore     bool
	shouldContainXG        bool
	shouldContainTimestamp bool
}

func TestDiscordNotifier_FormatMessage(t *testing.T) {
	testCases := []formatMessageTestCase{
		{
			name:                   "DifferentGoals_HomeWins",
			team1ID:                "CHI",
			team2ID:                "DET",
			homeGoals:              "3",
			awayGoals:              "1",
			homeXG:                 "2.5",
			awayXG:                 "1.2",
			homeShootOutGoals:      "0",
			awayShootOutGoals:      "0",
			expectedScore:          "CHI 3 - 1 DET",
			expectedHomeXG:         "CHI: 2.5",
			expectedAwayXG:         "DET: 1.2",
			shouldContainScore:     true,
			shouldContainXG:        true,
			shouldContainTimestamp: true,
		},
		{
			name:                   "DifferentGoals_AwayWins",
			team1ID:                "CHI",
			team2ID:                "DET",
			homeGoals:              "1",
			awayGoals:              "4",
			homeXG:                 "1.8",
			awayXG:                 "3.5",
			homeShootOutGoals:      "0",
			awayShootOutGoals:      "0",
			expectedScore:          "CHI 1 - 4 DET",
			expectedHomeXG:         "CHI: 1.8",
			expectedAwayXG:         "DET: 3.5",
			shouldContainScore:     true,
			shouldContainXG:        true,
			shouldContainTimestamp: true,
		},
		{
			name:                   "MissingShootoutData",
			team1ID:                "CHI",
			team2ID:                "DET",
			homeGoals:              "2",
			awayGoals:              "2",
			homeXG:                 "2.0",
			awayXG:                 "2.0",
			homeShootOutGoals:      "", // Missing shootout data
			awayShootOutGoals:      "",
			expectedScore:          "CHI 2 - 2 DET",
			expectedHomeXG:         "CHI: 2.0",
			expectedAwayXG:         "DET: 2.0",
			shouldContainScore:     true,
			shouldContainXG:        true,
			shouldContainTimestamp: true,
		},
		{
			name:                   "MissingExpectedGoals",
			team1ID:                "CHI",
			team2ID:                "DET",
			homeGoals:              "3",
			awayGoals:              "1",
			homeXG:                 "", // Missing xG data
			awayXG:                 "",
			homeShootOutGoals:      "0",
			awayShootOutGoals:      "0",
			expectedScore:          "CHI 3 - 1 DET",
			expectedHomeXG:         "",
			expectedAwayXG:         "",
			shouldContainScore:     true,
			shouldContainXG:        false,
			shouldContainTimestamp: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			discordNotifier := &DiscordNotifier{}
			req := buildNotificationRequest(tc)

			// Act
			message := discordNotifier.FormatMessage(req)

			// Assert
			assertMessageContent(t, message, tc)
		})
	}
}

// buildNotificationRequest constructs a NotificationRequest from test case data
func buildNotificationRequest(tc formatMessageTestCase) NotificationRequest {
	data := make(map[string]string)

	if tc.homeGoals != "" {
		data["homeTeamGoals"] = tc.homeGoals
	}
	if tc.awayGoals != "" {
		data["awayTeamGoals"] = tc.awayGoals
	}
	if tc.homeXG != "" {
		data["homeTeamExpectedGoals"] = tc.homeXG
	}
	if tc.awayXG != "" {
		data["awayTeamExpectedGoals"] = tc.awayXG
	}
	if tc.homeShootOutGoals != "" {
		data["homeTeamShootOutGoals"] = tc.homeShootOutGoals
	}
	if tc.awayShootOutGoals != "" {
		data["awayTeamShootOutGoals"] = tc.awayShootOutGoals
	}

	return NotificationRequest{
		Team1ID: tc.team1ID,
		Team2ID: tc.team2ID,
		Data:    data,
	}
}

// assertMessageContent validates the formatted message against expected content
func assertMessageContent(t *testing.T, message string, tc formatMessageTestCase) {
	t.Helper()

	// Assert score is present and correct
	if tc.shouldContainScore {
		if !strings.Contains(message, tc.expectedScore) {
			t.Errorf("Expected message to contain '%s', got: %s", tc.expectedScore, message)
		}
		if !strings.Contains(message, "🏒 Current Score:") {
			t.Errorf("Expected message to contain score header, got: %s", message)
		}
	}

	// Assert expected goals are present and correct
	if tc.shouldContainXG {
		if tc.expectedHomeXG != "" && !strings.Contains(message, tc.expectedHomeXG) {
			t.Errorf("Expected message to contain home team xG '%s', got: %s", tc.expectedHomeXG, message)
		}
		if tc.expectedAwayXG != "" && !strings.Contains(message, tc.expectedAwayXG) {
			t.Errorf("Expected message to contain away team xG '%s', got: %s", tc.expectedAwayXG, message)
		}
		if !strings.Contains(message, "📊 Expected Goals:") {
			t.Errorf("Expected message to contain xG header, got: %s", message)
		}
	}

	// Assert timestamp is present
	if tc.shouldContainTimestamp {
		if !strings.Contains(message, "*Notification sent at") {
			t.Errorf("Expected message to contain timestamp, got: %s", message)
		}
	}
}
