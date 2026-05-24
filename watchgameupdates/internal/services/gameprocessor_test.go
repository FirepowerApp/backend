package services

import (
	"testing"
	"time"

	"watchgameupdates/config"
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
			expectedHomeGoals: "3",
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
			expectedAwayGoals: "3",
			expectError:       false,
		},
		{
			name:              "TiedGoals_TiedShootout",
			homeGoals:         "3",
			awayGoals:         "3",
			homeShootOutGoals: "1",
			awayShootOutGoals: "1",
			expectedHomeGoals: "3",
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
			expectError:       true,
		},
		{
			name:              "InvalidAwayGoals",
			homeGoals:         "2",
			awayGoals:         "invalid",
			homeShootOutGoals: "1",
			awayShootOutGoals: "2",
			expectError:       true,
		},
		{
			name:              "InvalidHomeShootoutGoals",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "invalid",
			awayShootOutGoals: "2",
			expectError:       true,
		},
		{
			name:              "InvalidAwayShootoutGoals",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "1",
			awayShootOutGoals: "invalid",
			expectError:       true,
		},
		{
			name:              "MissingHomeGoals",
			homeGoals:         "",
			awayGoals:         "2",
			homeShootOutGoals: "1",
			awayShootOutGoals: "2",
			expectError:       true,
		},
		{
			name:              "MissingAwayGoals",
			homeGoals:         "2",
			awayGoals:         "",
			homeShootOutGoals: "1",
			awayShootOutGoals: "2",
			expectError:       true,
		},
		{
			name:              "MissingHomeShootoutGoals",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "",
			awayShootOutGoals: "2",
			expectError:       true,
		},
		{
			name:              "MissingAwayShootoutGoals",
			homeGoals:         "2",
			awayGoals:         "2",
			homeShootOutGoals: "1",
			awayShootOutGoals: "",
			expectError:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gameData := buildShootoutGameData(tc)

			err := AdjustScoreForShootout(gameData)

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

func buildShootoutGameData(tc adjustScoreTestCase) map[string]string {
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
			result := FormatGameState(tc.play)

			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestPeriodEndInterval(t *testing.T) {
	cfg := &config.Config{
		MessageIntervalSeconds:   60,
		PeriodEndIntervalSeconds: 1200,
	}
	maxPeriods := 5

	testCases := []struct {
		name       string
		period     int
		maxPeriods *int
		expected   time.Duration
	}{
		{
			name:       "RegularSeason_Period1_ReturnsExtendedInterval",
			period:     1,
			maxPeriods: &maxPeriods,
			expected:   1200 * time.Second,
		},
		{
			name:       "RegularSeason_Period2_ReturnsExtendedInterval",
			period:     2,
			maxPeriods: &maxPeriods,
			expected:   1200 * time.Second,
		},
		{
			name:       "RegularSeason_Period3_ReturnsStandardInterval",
			period:     3,
			maxPeriods: &maxPeriods,
			expected:   60 * time.Second,
		},
		{
			name:       "Playoffs_Period1_ReturnsExtendedInterval",
			period:     1,
			maxPeriods: nil,
			expected:   1200 * time.Second,
		},
		{
			name:       "Playoffs_Period2_ReturnsExtendedInterval",
			period:     2,
			maxPeriods: nil,
			expected:   1200 * time.Second,
		},
		{
			name:       "Playoffs_Period3_ReturnsExtendedInterval",
			period:     3,
			maxPeriods: nil,
			expected:   1200 * time.Second,
		},
		{
			name:       "Playoffs_OTPeriod4_ReturnsExtendedInterval",
			period:     4,
			maxPeriods: nil,
			expected:   1200 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			play := models.Play{
				TypeDescKey: "period-end",
				PeriodDescriptor: models.PeriodDescriptor{
					Number: tc.period,
				},
			}

			result := RescheduleInterval(play, tc.maxPeriods, cfg)

			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestShouldSkipExecution(t *testing.T) {
	t.Run("NilExecutionEnd_ShouldNotSkip", func(t *testing.T) {
		payload := models.Payload{
			Game: models.Game{ID: "2024030411"},
		}

		skip, err := ShouldSkipExecution(payload)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if skip {
			t.Error("Expected skip=false for nil ExecutionEnd")
		}
	})

	t.Run("FutureExecutionEnd_ShouldNotSkip", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &future,
		}

		skip, err := ShouldSkipExecution(payload)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if skip {
			t.Error("Expected skip=false for future ExecutionEnd")
		}
	})

	t.Run("PastExecutionEnd_ShouldSkip", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &past,
		}

		skip, err := ShouldSkipExecution(payload)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !skip {
			t.Error("Expected skip=true for past ExecutionEnd")
		}
	})

	t.Run("InvalidExecutionEnd_ReturnsError", func(t *testing.T) {
		invalid := "not-a-date"
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &invalid,
		}

		skip, err := ShouldSkipExecution(payload)
		if err == nil {
			t.Error("Expected error for invalid ExecutionEnd format")
		}
		if !skip {
			t.Error("Expected skip=true when ExecutionEnd is invalid")
		}
	})
}

func TestShouldReschedule(t *testing.T) {
	t.Run("NonGameEnd_ShouldReschedule", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &future,
		}
		lastPlay := models.Play{TypeDescKey: "shot-on-goal"}

		result := ShouldReschedule(payload, lastPlay)
		if !result {
			t.Error("Expected ShouldReschedule=true for non game-end play")
		}
	})

	t.Run("GameEnd_ShouldNotReschedule", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &future,
		}
		lastPlay := models.Play{TypeDescKey: "game-end"}

		result := ShouldReschedule(payload, lastPlay)
		if result {
			t.Error("Expected ShouldReschedule=false for game-end play")
		}
	})

	t.Run("PastExecutionEnd_ShouldNotReschedule", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
		payload := models.Payload{
			Game:         models.Game{ID: "2024030411"},
			ExecutionEnd: &past,
		}
		lastPlay := models.Play{TypeDescKey: "shot-on-goal"}

		result := ShouldReschedule(payload, lastPlay)
		if result {
			t.Error("Expected ShouldReschedule=false when execution end has passed")
		}
	})
}
