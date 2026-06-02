package config

import (
	"reflect"
	"testing"
)

func TestParseTeamFilter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string returns nil (monitor all)",
			input: "",
			want:  nil,
		},
		{
			name:  "single value",
			input: "DAL",
			want:  []string{"DAL"},
		},
		{
			name:  "multi value",
			input: "DAL,COL",
			want:  []string{"DAL", "COL"},
		},
		{
			name:  "trims whitespace and uppercases",
			input: "dal, Col ",
			want:  []string{"DAL", "COL"},
		},
		{
			name:  "deduplicates",
			input: "DAL,DAL,COL",
			want:  []string{"DAL", "COL"},
		},
		{
			name:  "all commas and spaces returns nil (monitor all)",
			input: " , , ",
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseTeamFilter(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseTeamFilter(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
