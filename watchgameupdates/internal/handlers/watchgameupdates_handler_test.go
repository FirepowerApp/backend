package handlers

import (
	"testing"

	"watchgameupdates/internal/models"
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

func TestFormatGameState(t *testing.T) {
	testCases := []struct {
		name     string
		play     models.Play
		expected string
	}{
		{
			name: "GameEnd_ReturnsFinal",
			play: models.Play{
				TypeDescKey:   "game-end",
				TimeRemaining: "00:00",
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     3,
					PeriodType: "REG",
				},
			},
			expected: "Final",
		},
		{
			name: "FirstPeriod_ReturnsCorrectSuffix",
			play: models.Play{
				TypeDescKey:   "shot-on-goal",
				TimeRemaining: "15:32",
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     1,
					PeriodType: "REG",
				},
			},
			expected: "15:32 left, 1st period",
		},
		{
			name: "SecondPeriod_ReturnsCorrectSuffix",
			play: models.Play{
				TypeDescKey:   "goal",
				TimeRemaining: "06:56",
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     2,
					PeriodType: "REG",
				},
			},
			expected: "06:56 left, 2nd period",
		},
		{
			name: "ThirdPeriod_ReturnsCorrectSuffix",
			play: models.Play{
				TypeDescKey:   "missed-shot",
				TimeRemaining: "01:45",
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     3,
					PeriodType: "REG",
				},
			},
			expected: "01:45 left, 3rd period",
		},
		{
			name: "FourthPeriod_ReturnsFallbackSuffix",
			play: models.Play{
				TypeDescKey:   "blocked-shot",
				TimeRemaining: "10:00",
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     4,
					PeriodType: "REG",
				},
			},
			expected: "10:00 left, 4th period",
		},
		{
			name: "Overtime_ReturnsOTFormat",
			play: models.Play{
				TypeDescKey:   "shot-on-goal",
				TimeRemaining: "03:22",
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     4,
					PeriodType: "OT",
				},
			},
			expected: "03:22 left, OT",
		},
		{
			name: "Shootout_ReturnsShootout",
			play: models.Play{
				TypeDescKey:   "goal",
				TimeRemaining: "00:00",
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     5,
					PeriodType: "SO",
				},
			},
			expected: "Shootout",
		},
		{
			name: "EmptyTimeRemaining_ReturnsEmptyString",
			play: models.Play{
				TypeDescKey:   "period-start",
				TimeRemaining: "",
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     1,
					PeriodType: "REG",
				},
			},
			expected: "",
		},
		{
			name: "PeriodDescriptorNumber_UsedInsteadOfPeriodField",
			play: models.Play{
				TypeDescKey:   "shot-on-goal",
				TimeRemaining: "12:00",
				Period:        0, // This field is not populated by NHL API
				PeriodDescriptor: models.PeriodDescriptor{
					Number:     2, // This is what the NHL API populates
					PeriodType: "REG",
				},
			},
			expected: "12:00 left, 2nd period",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatGameState(tc.play)

			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}
