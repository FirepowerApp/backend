package handlers

import (
	"testing"
)

type adjustScoreTestCase struct {
	name              string
	homeGoals         string
	awayGoals         string
	homeShootOutGoals string
	awayShootOutGoals string
	expectedHomeGoals string
	expectedAwayGoals string
	expectError       bool
}

func TestAdjustScoreForShootout(t *testing.T) {
	testCases := []adjustScoreTestCase{
		{
			name:              "TiedGoals_HomeWinsShootout",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "2",
			awayShootOutGoals: "1",
			expectedHomeGoals: "3", // Home gets +1 for shootout win
			expectedAwayGoals: "2",
			expectError:       false,
		},
		{
			name:              "TiedGoals_AwayWinsShootout",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "1",
			awayShootOutGoals: "3",
			expectedHomeGoals: "2",
			expectedAwayGoals: "3", // Away gets +1 for shootout win
			expectError:       false,
		},
		{
			name:              "TiedGoals_TiedShootout",
			homeGoals:         "3",
			awayGoals:         "3",
			homeShootOutGoals: "1",
			awayShootOutGoals: "1",
			expectedHomeGoals: "3", // No change for tied shootout
			expectedAwayGoals: "3",
			expectError:       false,
		},
		{
			name:              "DifferentInitialScores_HomeWinsShootout",
			homeGoals:         "1",
			awayGoals:         "1",
			homeShootOutGoals: "3",
			awayShootOutGoals: "2",
			expectedHomeGoals: "2",
			expectedAwayGoals: "1",
			expectError:       false,
		},
		{
			name:              "HighScoreTie_AwayWinsShootout",
			homeGoals:         "5",
			awayGoals:         "5",
			homeShootOutGoals: "0",
			awayShootOutGoals: "1",
			expectedHomeGoals: "5",
			expectedAwayGoals: "6",
			expectError:       false,
		},
		{
			name:              "InvalidHomeGoals",
			homeGoals:         "invalid",
			awayGoals:         "2",
			homeShootOutGoals: "1",
			awayShootOutGoals: "2",
			expectedHomeGoals: "",
			expectedAwayGoals: "",
			expectError:       true,
		},
		{
			name:              "InvalidAwayGoals",
			homeGoals:         "2",
			awayGoals:         "invalid",
			homeShootOutGoals: "1",
			awayShootOutGoals: "2",
			expectedHomeGoals: "",
			expectedAwayGoals: "",
			expectError:       true,
		},
		{
			name:              "InvalidHomeShootoutGoals",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "invalid",
			awayShootOutGoals: "2",
			expectedHomeGoals: "",
			expectedAwayGoals: "",
			expectError:       true,
		},
		{
			name:              "InvalidAwayShootoutGoals",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "1",
			awayShootOutGoals: "invalid",
			expectedHomeGoals: "",
			expectedAwayGoals: "",
			expectError:       true,
		},
		{
			name:              "MissingHomeGoals",
			homeGoals:         "",
			awayGoals:         "2",
			homeShootOutGoals: "1",
			awayShootOutGoals: "2",
			expectedHomeGoals: "",
			expectedAwayGoals: "",
			expectError:       true,
		},
		{
			name:              "MissingAwayGoals",
			homeGoals:         "2",
			awayGoals:         "",
			homeShootOutGoals: "1",
			awayShootOutGoals: "2",
			expectedHomeGoals: "",
			expectedAwayGoals: "",
			expectError:       true,
		},
		{
			name:              "MissingHomeShootoutGoals",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "",
			awayShootOutGoals: "2",
			expectedHomeGoals: "",
			expectedAwayGoals: "",
			expectError:       true,
		},
		{
			name:              "MissingAwayShootoutGoals",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "1",
			awayShootOutGoals: "",
			expectedHomeGoals: "",
			expectedAwayGoals: "",
			expectError:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			gameData := buildGameData(tc)

			// Act
			err := adjustScoreForShootout(gameData)

			// Assert
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				actualHomeGoals := gameData["homeTeamGoals"]
				actualAwayGoals := gameData["awayTeamGoals"]

				if actualHomeGoals != tc.expectedHomeGoals {
					t.Errorf("Expected homeTeamGoals to be '%s', got '%s'", tc.expectedHomeGoals, actualHomeGoals)
				}

				if actualAwayGoals != tc.expectedAwayGoals {
					t.Errorf("Expected awayTeamGoals to be '%s', got '%s'", tc.expectedAwayGoals, actualAwayGoals)
				}
			}
		})
	}
}

// buildGameData constructs a gameData map from test case data
func buildGameData(tc adjustScoreTestCase) map[string]string {
	data := make(map[string]string)

	if tc.homeGoals != "" {
		data["homeTeamGoals"] = tc.homeGoals
	}
	if tc.awayGoals != "" {
		data["awayTeamGoals"] = tc.awayGoals
	}
	if tc.homeShootOutGoals != "" {
		data["homeTeamShootOutGoals"] = tc.homeShootOutGoals
	}
	if tc.awayShootOutGoals != "" {
		data["awayTeamShootOutGoals"] = tc.awayShootOutGoals
	}

	return data
}
