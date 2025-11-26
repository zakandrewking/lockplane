package cmd

import (
	"fmt"
	"os"

	"github.com/lockplane/lockplane/internal/validation"
)

// printValidationReport renders a detailed safety report to stderr.
func printValidationReport(results []validation.ValidationResult, heading string) {
	if len(results) == 0 {
		return
	}

	if heading == "" {
		heading = "=== Migration Safety Report ==="
	}

	fmt.Fprintf(os.Stderr, "\n%s\n\n", heading)

	for i, result := range results {
		if result.Safety != nil {
			fmt.Fprintf(os.Stderr, "%s %s", result.Safety.Level.Icon(), result.Safety.Level.String())
			if result.Valid {
				fmt.Fprintf(os.Stderr, " (Operation %d)\n", i+1)
			} else {
				fmt.Fprintf(os.Stderr, " - BLOCKED (Operation %d)\n", i+1)
			}
		} else if result.Valid {
			fmt.Fprintf(os.Stderr, "✓ Operation %d: PASS\n", i+1)
		} else {
			fmt.Fprintf(os.Stderr, "✗ Operation %d: FAIL\n", i+1)
		}

		if result.Safety != nil {
			if result.Safety.BreakingChange {
				fmt.Fprintf(os.Stderr, "  ⚠️  Breaking change - will affect running applications\n")
			}
			if result.Safety.DataLoss {
				fmt.Fprintf(os.Stderr, "  💥 Permanent data loss\n")
			}
			if !result.Reversible && result.Safety.RollbackDescription != "" {
				fmt.Fprintf(os.Stderr, "  ↩️  Rollback: %s\n", result.Safety.RollbackDescription)
			} else if result.Reversible && result.Safety.RollbackDataLoss {
				fmt.Fprintf(os.Stderr, "  ↩️  Rollback: %s\n", result.Safety.RollbackDescription)
			}
		} else if !result.Reversible {
			fmt.Fprintf(os.Stderr, "  ⚠️  NOT REVERSIBLE\n")
		}

		for _, err := range result.Errors {
			fmt.Fprintf(os.Stderr, "  ❌ Error: %s\n", err)
		}
		for _, warning := range result.Warnings {
			fmt.Fprintf(os.Stderr, "  ⚠️  Warning: %s\n", warning)
		}

		if result.Safety != nil && len(result.Safety.SaferAlternatives) > 0 {
			fmt.Fprintf(os.Stderr, "\n  💡 Safer alternatives:\n")
			for _, alt := range result.Safety.SaferAlternatives {
				fmt.Fprintf(os.Stderr, "     • %s\n", alt)
			}
		}

		fmt.Fprintf(os.Stderr, "\n")
	}

	fmt.Fprintf(os.Stderr, "=== Summary ===\n\n")

	safeCnt, reviewCnt, lossyCnt, dangerousCnt, multiPhaseCnt := 0, 0, 0, 0, 0
	for _, r := range results {
		if r.Safety == nil {
			continue
		}
		switch r.Safety.Level {
		case validation.SafetyLevelSafe:
			safeCnt++
		case validation.SafetyLevelReview:
			reviewCnt++
		case validation.SafetyLevelLossy:
			lossyCnt++
		case validation.SafetyLevelDangerous:
			dangerousCnt++
		case validation.SafetyLevelMultiPhase:
			multiPhaseCnt++
		}
	}

	if safeCnt > 0 {
		fmt.Fprintf(os.Stderr, "  ✅ %d safe operation(s)\n", safeCnt)
	}
	if reviewCnt > 0 {
		fmt.Fprintf(os.Stderr, "  ⚠️  %d operation(s) require review\n", reviewCnt)
	}
	if lossyCnt > 0 {
		fmt.Fprintf(os.Stderr, "  🔶 %d lossy operation(s)\n", lossyCnt)
	}
	if dangerousCnt > 0 {
		fmt.Fprintf(os.Stderr, "  ❌ %d dangerous operation(s)\n", dangerousCnt)
	}
	if multiPhaseCnt > 0 {
		fmt.Fprintf(os.Stderr, "  🔄 %d operation(s) require multi-phase migration\n", multiPhaseCnt)
	}

	fmt.Fprintf(os.Stderr, "\n")
}
