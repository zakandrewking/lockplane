package main

import (
	"strings"
)

// levenshteinDistance calculates the Levenshtein distance between two strings
// using a dynamic programming approach with space optimization
func levenshteinDistance(s1, s2 string) int {
	s1Lower := strings.ToLower(s1)
	s2Lower := strings.ToLower(s2)

	if len(s1Lower) == 0 {
		return len(s2Lower)
	}
	if len(s2Lower) == 0 {
		return len(s1Lower)
	}

	// Use two rows instead of full matrix for space efficiency
	prev := make([]int, len(s2Lower)+1)
	curr := make([]int, len(s2Lower)+1)

	// Initialize first row
	for j := 0; j <= len(s2Lower); j++ {
		prev[j] = j
	}

	// Calculate distances
	for i := 1; i <= len(s1Lower); i++ {
		curr[0] = i
		for j := 1; j <= len(s2Lower); j++ {
			cost := 1
			if s1Lower[i-1] == s2Lower[j-1] {
				cost = 0
			}

			curr[j] = min3(
				curr[j-1]+1,    // insertion
				prev[j]+1,      // deletion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(s2Lower)]
}

// findClosestCommand finds the closest command to the given input
// Returns the closest command and its distance, or empty string if no close match
func findClosestCommand(input string, validCommands []string, maxDistance int) (string, int) {
	if len(validCommands) == 0 {
		return "", -1
	}

	closestCmd := ""
	minDistance := maxDistance + 1

	for _, cmd := range validCommands {
		distance := levenshteinDistance(input, cmd)
		if distance < minDistance {
			minDistance = distance
			closestCmd = cmd
		}
	}

	if minDistance <= maxDistance {
		return closestCmd, minDistance
	}

	return "", minDistance
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
