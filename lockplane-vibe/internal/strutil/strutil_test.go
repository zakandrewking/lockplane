package strutil

import (
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		{
			name:     "identical strings",
			s1:       "apply",
			s2:       "apply",
			expected: 0,
		},
		{
			name:     "single character difference",
			s1:       "applly",
			s2:       "apply",
			expected: 1,
		},
		{
			name:     "case insensitive - uppercase",
			s1:       "APPLY",
			s2:       "apply",
			expected: 0,
		},
		{
			name:     "case insensitive - mixed case",
			s1:       "ApPlY",
			s2:       "apply",
			expected: 0,
		},
		{
			name:     "one character insertion",
			s1:       "aply",
			s2:       "apply",
			expected: 1,
		},
		{
			name:     "one character deletion",
			s1:       "applyy",
			s2:       "apply",
			expected: 1,
		},
		{
			name:     "one character substitution",
			s1:       "appla",
			s2:       "apply",
			expected: 1,
		},
		{
			name:     "two character difference",
			s1:       "aplli",
			s2:       "apply",
			expected: 2,
		},
		{
			name:     "completely different strings",
			s1:       "xyz",
			s2:       "apply",
			expected: 5,
		},
		{
			name:     "empty string",
			s1:       "",
			s2:       "apply",
			expected: 5,
		},
		{
			name:     "both empty",
			s1:       "",
			s2:       "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := LevenshteinDistance(tt.s1, tt.s2)
			if distance != tt.expected {
				t.Errorf("LevenshteinDistance(%q, %q) = %d; want %d",
					tt.s1, tt.s2, distance, tt.expected)
			}
		})
	}
}

func TestFindClosestCommand(t *testing.T) {
	validCommands := []string{
		"init", "introspect", "diff", "plan", "rollback",
		"apply", "validate", "convert", "version", "help",
	}

	tests := []struct {
		name            string
		input           string
		maxDistance     int
		expectedCmd     string
		expectedMatches bool
	}{
		{
			name:            "exact match",
			input:           "apply",
			maxDistance:     2,
			expectedCmd:     "apply",
			expectedMatches: true,
		},
		{
			name:            "one typo - applly",
			input:           "applly",
			maxDistance:     2,
			expectedCmd:     "apply",
			expectedMatches: true,
		},
		{
			name:            "one typo - aply",
			input:           "aply",
			maxDistance:     2,
			expectedCmd:     "apply",
			expectedMatches: true,
		},
		{
			name:            "one typo - appl",
			input:           "appl",
			maxDistance:     2,
			expectedCmd:     "apply",
			expectedMatches: true,
		},
		{
			name:            "case insensitive - APPLY",
			input:           "APPLY",
			maxDistance:     2,
			expectedCmd:     "apply",
			expectedMatches: true,
		},
		{
			name:            "two typos - aplly",
			input:           "aplly",
			maxDistance:     2,
			expectedCmd:     "apply",
			expectedMatches: true,
		},
		{
			name:            "close to introspect",
			input:           "introspct",
			maxDistance:     2,
			expectedCmd:     "introspect",
			expectedMatches: true,
		},
		{
			name:            "close to validate",
			input:           "validat",
			maxDistance:     2,
			expectedCmd:     "validate",
			expectedMatches: true,
		},
		{
			name:            "close to rollback",
			input:           "rolback",
			maxDistance:     2,
			expectedCmd:     "rollback",
			expectedMatches: true,
		},
		{
			name:            "too far from any command",
			input:           "xyz",
			maxDistance:     2,
			expectedCmd:     "",
			expectedMatches: false,
		},
		{
			name:            "one typo - xpply (distance 2)",
			input:           "xpply",
			maxDistance:     2,
			expectedCmd:     "apply",
			expectedMatches: true,
		},
		{
			name:            "strict maxDistance - no match",
			input:           "applly",
			maxDistance:     0,
			expectedCmd:     "",
			expectedMatches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _ := FindClosestCommand(tt.input, validCommands, tt.maxDistance)
			if tt.expectedMatches {
				if cmd != tt.expectedCmd {
					t.Errorf("FindClosestCommand(%q) = %q; want %q",
						tt.input, cmd, tt.expectedCmd)
				}
			} else {
				if cmd != "" {
					t.Errorf("FindClosestCommand(%q) = %q; want no match (empty string)",
						tt.input, cmd)
				}
			}
		})
	}
}

func TestFindClosestCommandDistance(t *testing.T) {
	validCommands := []string{"apply", "plan"}

	tests := []struct {
		name             string
		input            string
		maxDistance      int
		expectedDistance int
	}{
		{
			name:             "exact match - distance 0",
			input:            "apply",
			maxDistance:      2,
			expectedDistance: 0,
		},
		{
			name:             "one typo - distance 1",
			input:            "applly",
			maxDistance:      2,
			expectedDistance: 1,
		},
		{
			name:             "one typo - distance 1 (aplly -> apply)",
			input:            "aplly",
			maxDistance:      2,
			expectedDistance: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, distance := FindClosestCommand(tt.input, validCommands, tt.maxDistance)
			if distance != tt.expectedDistance {
				t.Errorf("FindClosestCommand(%q) distance = %d; want %d",
					tt.input, distance, tt.expectedDistance)
			}
		})
	}
}
