package main

import (
	"fmt"
	"regexp"
	"strings"
)

// SQL parsing utilities for extracting identifiers from SQL statements
// These are simplified parsers that work for the SQL we generate

// extractTableNameFromCreate extracts table name from CREATE TABLE statement
func extractTableNameFromCreate(sql string) (string, error) {
	// Pattern: CREATE TABLE <name> ...
	re := regexp.MustCompile(`CREATE\s+TABLE\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract table name from: %s", sql)
	}
	return matches[1], nil
}

// extractTableNameFromDrop extracts table name from DROP TABLE statement
func extractTableNameFromDrop(sql string) (string, error) {
	// Pattern: DROP TABLE <name> [CASCADE]
	re := regexp.MustCompile(`DROP\s+TABLE\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract table name from: %s", sql)
	}
	return matches[1], nil
}

// extractTableAndColumnFromAddColumn extracts table and column name from ALTER TABLE ADD COLUMN
func extractTableAndColumnFromAddColumn(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ADD COLUMN <column> ...
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromDropColumn extracts table and column name from ALTER TABLE DROP COLUMN
func extractTableAndColumnFromDropColumn(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> DROP COLUMN <column>
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+DROP\s+COLUMN\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromAlterType extracts table and column from ALTER COLUMN TYPE
func extractTableAndColumnFromAlterType(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ALTER COLUMN <column> TYPE <type>
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+TYPE`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromAlterNotNull extracts table and column from ALTER COLUMN SET/DROP NOT NULL
func extractTableAndColumnFromAlterNotNull(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ALTER COLUMN <column> SET/DROP NOT NULL
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+(SET|DROP)\s+NOT\s+NULL`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromSetDefault extracts table and column from SET DEFAULT
func extractTableAndColumnFromSetDefault(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ALTER COLUMN <column> SET DEFAULT ...
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+SET\s+DEFAULT`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromDropDefault extracts table and column from DROP DEFAULT
func extractTableAndColumnFromDropDefault(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ALTER COLUMN <column> DROP DEFAULT
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+DROP\s+DEFAULT`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractIndexNameFromCreate extracts index name from CREATE INDEX
func extractIndexNameFromCreate(sql string) (string, error) {
	// Pattern: CREATE [UNIQUE] INDEX <name> ON ...
	re := regexp.MustCompile(`CREATE\s+(UNIQUE\s+)?INDEX\s+(\w+)\s+ON`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", fmt.Errorf("could not extract index name from: %s", sql)
	}
	return matches[2], nil
}

// extractIndexNameFromDrop extracts index name from DROP INDEX
func extractIndexNameFromDrop(sql string) (string, error) {
	// Pattern: DROP INDEX <name>
	re := regexp.MustCompile(`DROP\s+INDEX\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract index name from: %s", sql)
	}
	return matches[1], nil
}

// extractTableAndConstraintFromAddConstraint extracts table and constraint name from ADD CONSTRAINT
func extractTableAndConstraintFromAddConstraint(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ADD CONSTRAINT <constraint> ...
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ADD\s+CONSTRAINT\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and constraint from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndConstraintFromDropConstraint extracts table and constraint name from DROP CONSTRAINT
func extractTableAndConstraintFromDropConstraint(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> DROP CONSTRAINT <constraint>
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+DROP\s+CONSTRAINT\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and constraint from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// containsSQL is a helper to check if SQL contains a substring (case-insensitive)
func containsSQL(sql, substr string) bool {
	return strings.Contains(strings.ToUpper(sql), strings.ToUpper(substr))
}
