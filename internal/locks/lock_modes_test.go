package locks

import "testing"

func TestLockMode_String(t *testing.T) {
	tests := []struct {
		mode     LockMode
		expected string
	}{
		{LockAccessShare, "ACCESS SHARE"},
		{LockRowShare, "ROW SHARE"},
		{LockRowExclusive, "ROW EXCLUSIVE"},
		{LockShareUpdateExclusive, "SHARE UPDATE EXCLUSIVE"},
		{LockShare, "SHARE"},
		{LockShareRowExclusive, "SHARE ROW EXCLUSIVE"},
		{LockExclusive, "EXCLUSIVE"},
		{LockAccessExclusive, "ACCESS EXCLUSIVE"},
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
		mode     LockMode
		expected bool
	}{
		{LockAccessShare, false},
		{LockRowShare, false},
		{LockRowExclusive, false},
		{LockShareUpdateExclusive, false},
		{LockShare, false},
		{LockShareRowExclusive, false},
		{LockExclusive, false},
		{LockAccessExclusive, true}, // Only ACCESS EXCLUSIVE blocks reads
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
		mode     LockMode
		expected bool
	}{
		{LockAccessShare, false},
		{LockRowShare, false},
		{LockRowExclusive, false},
		{LockShareUpdateExclusive, false},
		{LockShare, true}, // SHARE and above block writes
		{LockShareRowExclusive, true},
		{LockExclusive, true},
		{LockAccessExclusive, true},
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
		mode     LockMode
		expected ImpactLevel
	}{
		{LockAccessShare, ImpactNone},
		{LockRowShare, ImpactNone},
		{LockRowExclusive, ImpactNone},
		{LockShareUpdateExclusive, ImpactLow},
		{LockShare, ImpactMedium},
		{LockShareRowExclusive, ImpactHigh},
		{LockExclusive, ImpactHigh},
		{LockAccessExclusive, ImpactHigh},
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
		impact   ImpactLevel
		expected string
	}{
		{ImpactNone, "NONE"},
		{ImpactLow, "LOW"},
		{ImpactMedium, "MEDIUM"},
		{ImpactHigh, "HIGH"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.impact.String(); got != tt.expected {
				t.Errorf("ImpactLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestImpactLevel_Emoji(t *testing.T) {
	tests := []struct {
		impact ImpactLevel
		emoji  string
	}{
		{ImpactNone, "✓"},
		{ImpactLow, "⚡"},
		{ImpactMedium, "⚠️"},
		{ImpactHigh, "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.impact.String(), func(t *testing.T) {
			got := tt.impact.Emoji()
			if got != tt.emoji {
				t.Errorf("ImpactLevel.Emoji() = %v, want %v", got, tt.emoji)
			}
		})
	}
}

func TestLockImpact_IsHighImpact(t *testing.T) {
	tests := []struct {
		name     string
		impact   ImpactLevel
		expected bool
	}{
		{"None impact", ImpactNone, false},
		{"Low impact", ImpactLow, false},
		{"Medium impact", ImpactMedium, true},
		{"High impact", ImpactHigh, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := &LockImpact{Impact: tt.impact}
			if got := li.IsHighImpact(); got != tt.expected {
				t.Errorf("LockImpact.IsHighImpact() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLockImpact_RequiresSaferAlternative(t *testing.T) {
	tests := []struct {
		name                string
		impact              ImpactLevel
		estimatedDurationMS int64
		expected            bool
	}{
		{"Low impact, fast", ImpactLow, 100, false},
		{"Low impact, slow", ImpactLow, 2000, true},
		{"Medium impact, fast", ImpactMedium, 100, true},
		{"Medium impact, slow", ImpactMedium, 2000, true},
		{"High impact, fast", ImpactHigh, 100, true},
		{"None impact, very slow", ImpactNone, 5000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := &LockImpact{
				Impact:              tt.impact,
				EstimatedDurationMS: tt.estimatedDurationMS,
			}
			if got := li.RequiresSaferAlternative(); got != tt.expected {
				t.Errorf("LockImpact.RequiresSaferAlternative() = %v, want %v", got, tt.expected)
			}
		})
	}
}
