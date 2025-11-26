package locks_test

import (
	"testing"

	"github.com/lockplane/lockplane/internal/locks"
)

func TestLockMode_String(t *testing.T) {
	tests := []struct {
		mode     locks.LockMode
		expected string
	}{
		{locks.LockAccessShare, "ACCESS SHARE"},
		{locks.LockRowShare, "ROW SHARE"},
		{locks.LockRowExclusive, "ROW EXCLUSIVE"},
		{locks.LockShareUpdateExclusive, "SHARE UPDATE EXCLUSIVE"},
		{locks.LockShare, "SHARE"},
		{locks.LockShareRowExclusive, "SHARE ROW EXCLUSIVE"},
		{locks.LockExclusive, "EXCLUSIVE"},
		{locks.LockAccessExclusive, "ACCESS EXCLUSIVE"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.expected {
				t.Errorf("LockMode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLockMode_BlocksReads(t *testing.T) {
	tests := []struct {
		mode     locks.LockMode
		expected bool
	}{
		{locks.LockAccessShare, false},
		{locks.LockRowShare, false},
		{locks.LockRowExclusive, false},
		{locks.LockShareUpdateExclusive, false},
		{locks.LockShare, false},
		{locks.LockShareRowExclusive, false},
		{locks.LockExclusive, false},
		{locks.LockAccessExclusive, true}, // Only ACCESS EXCLUSIVE blocks reads
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			if got := tt.mode.BlocksReads(); got != tt.expected {
				t.Errorf("LockMode.BlocksReads() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLockMode_BlocksWrites(t *testing.T) {
	tests := []struct {
		mode     locks.LockMode
		expected bool
	}{
		{locks.LockAccessShare, false},
		{locks.LockRowShare, false},
		{locks.LockRowExclusive, false},
		{locks.LockShareUpdateExclusive, false},
		{locks.LockShare, true}, // SHARE and above block writes
		{locks.LockShareRowExclusive, true},
		{locks.LockExclusive, true},
		{locks.LockAccessExclusive, true},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			if got := tt.mode.BlocksWrites(); got != tt.expected {
				t.Errorf("LockMode.BlocksWrites() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLockMode_ImpactLevel(t *testing.T) {
	tests := []struct {
		mode     locks.LockMode
		expected locks.ImpactLevel
	}{
		{locks.LockAccessShare, locks.ImpactNone},
		{locks.LockRowShare, locks.ImpactNone},
		{locks.LockRowExclusive, locks.ImpactNone},
		{locks.LockShareUpdateExclusive, locks.ImpactLow},
		{locks.LockShare, locks.ImpactMedium},
		{locks.LockShareRowExclusive, locks.ImpactHigh},
		{locks.LockExclusive, locks.ImpactHigh},
		{locks.LockAccessExclusive, locks.ImpactHigh},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			if got := tt.mode.ImpactLevel(); got != tt.expected {
				t.Errorf("LockMode.ImpactLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestImpactLevel_String(t *testing.T) {
	tests := []struct {
		impact   locks.ImpactLevel
		expected string
	}{
		{locks.ImpactNone, "NONE"},
		{locks.ImpactLow, "LOW"},
		{locks.ImpactMedium, "MEDIUM"},
		{locks.ImpactHigh, "HIGH"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.impact.String(); got != tt.expected {
				t.Errorf("locks.ImpactLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestImpactLevel_Emoji(t *testing.T) {
	tests := []struct {
		impact locks.ImpactLevel
		emoji  string
	}{
		{locks.ImpactNone, "✓"},
		{locks.ImpactLow, "⚡"},
		{locks.ImpactMedium, "⚠️"},
		{locks.ImpactHigh, "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.impact.String(), func(t *testing.T) {
			got := tt.impact.Emoji()
			if got != tt.emoji {
				t.Errorf("locks.ImpactLevel.Emoji() = %v, want %v", got, tt.emoji)
			}
		})
	}
}

func TestLockImpact_IsHighImpact(t *testing.T) {
	tests := []struct {
		name     string
		impact   locks.ImpactLevel
		expected bool
	}{
		{"None impact", locks.ImpactNone, false},
		{"Low impact", locks.ImpactLow, false},
		{"Medium impact", locks.ImpactMedium, true},
		{"High impact", locks.ImpactHigh, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := &locks.LockImpact{Impact: tt.impact}
			if got := li.IsHighImpact(); got != tt.expected {
				t.Errorf("locks.LockImpact.IsHighImpact() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLockImpact_RequiresSaferAlternative(t *testing.T) {
	tests := []struct {
		name                string
		impact              locks.ImpactLevel
		estimatedDurationMS int64
		expected            bool
	}{
		{"Low impact, fast", locks.ImpactLow, 100, false},
		{"Low impact, slow", locks.ImpactLow, 2000, true},
		{"Medium impact, fast", locks.ImpactMedium, 100, true},
		{"Medium impact, slow", locks.ImpactMedium, 2000, true},
		{"High impact, fast", locks.ImpactHigh, 100, true},
		{"None impact, very slow", locks.ImpactNone, 5000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := &locks.LockImpact{
				Impact:              tt.impact,
				EstimatedDurationMS: tt.estimatedDurationMS,
			}
			if got := li.RequiresSaferAlternative(); got != tt.expected {
				t.Errorf("locks.LockImpact.RequiresSaferAlternative() = %v, want %v", got, tt.expected)
			}
		})
	}
}
