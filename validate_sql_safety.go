package main

import (
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// validateDangerousPatterns detects dangerous SQL patterns that are syntactically
// valid but operationally risky in production migrations
func validateDangerousPatterns(filePath string, sqlContent string) []ValidationIssue {
	var issues []ValidationIssue

	// Parse the SQL
	tree, err := pg_query.Parse(sqlContent)
	if err != nil {
		// If it doesn't parse, syntax validation will catch it
		return issues
	}

	// Walk through all statements
	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}

		// Convert byte offset to line number
		lineNum := getLineNumber(sqlContent, int(stmt.StmtLocation))

		stmtIssues := detectDataLossOperations(filePath, stmt.Stmt, lineNum)
		issues = append(issues, stmtIssues...)

		nonDeclarativeIssues := detectNonDeclarativePatterns(filePath, stmt.Stmt, lineNum)
		issues = append(issues, nonDeclarativeIssues...)
	}

	return issues
}

// getLineNumber converts a byte offset into a line number (1-based)
func getLineNumber(content string, offset int) int {
	if offset < 0 || offset > len(content) {
		return 1
	}

	// Count newlines up to the offset
	lineNum := 1
	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			lineNum++
		}
	}

	// If we're at a newline or whitespace, skip forward to find the actual statement start
	// This handles cases where StmtLocation points to whitespace before the statement
	for offset < len(content) && (content[offset] == '\n' || content[offset] == ' ' || content[offset] == '\t' || content[offset] == '\r') {
		if content[offset] == '\n' {
			lineNum++
		}
		offset++
	}

	return lineNum
}

// detectDataLossOperations detects operations that irreversibly delete data
func detectDataLossOperations(filePath string, stmt *pg_query.Node, line int) []ValidationIssue {
	var issues []ValidationIssue

	switch node := stmt.Node.(type) {
	case *pg_query.Node_DropStmt:
		// DROP TABLE, DROP SCHEMA, etc.
		dropStmt := node.DropStmt

		if dropStmt.RemoveType == pg_query.ObjectType_OBJECT_TABLE {
			tableName := extractObjectName(dropStmt.Objects)
			cascade := ""
			if dropStmt.Behavior == pg_query.DropBehavior_DROP_CASCADE {
				cascade = " CASCADE"
			}

			issues = append(issues, ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   1,
				Severity: "error",
				Message: fmt.Sprintf("DROP TABLE is not allowed in .lp.sql schema files\n"+
					"  Found: DROP TABLE %s%s\n"+
					"  Why: .lp.sql files are declarative schema definitions (CREATE only)\n"+
					"       Destructive operations like DROP are handled by Lockplane migrations\n"+
					"  To drop a table: Remove it from your schema, then run 'lockplane apply'\n"+
					"  Learn more: https://github.com/zakandrewking/lockplane#schema-definition-formats",
					tableName, cascade),
				Code: "dangerous_drop_table",
			})
		}

	case *pg_query.Node_TruncateStmt:
		// TRUNCATE TABLE
		truncateStmt := node.TruncateStmt
		tableNames := extractRelationNames(truncateStmt.Relations)

		issues = append(issues, ValidationIssue{
			File:     filePath,
			Line:     line,
			Column:   1,
			Severity: "error",
			Message: fmt.Sprintf("TRUNCATE is not allowed in .lp.sql schema files\n"+
				"  Found: TRUNCATE TABLE %s\n"+
				"  Why: .lp.sql files are declarative schema definitions (CREATE only)\n"+
				"       Data operations like TRUNCATE are not part of schema definitions\n"+
				"  Learn more: https://github.com/zakandrewking/lockplane#schema-definition-formats",
				strings.Join(tableNames, ", ")),
			Code: "dangerous_truncate",
		})

	case *pg_query.Node_DeleteStmt:
		// DELETE without WHERE clause
		deleteStmt := node.DeleteStmt

		if deleteStmt.WhereClause == nil {
			tableName := extractRangeVarName(deleteStmt.Relation)

			issues = append(issues, ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   1,
				Severity: "error",
				Message: fmt.Sprintf("DELETE is not allowed in .lp.sql schema files\n"+
					"  Found: DELETE FROM %s\n"+
					"  Why: .lp.sql files are declarative schema definitions (CREATE only)\n"+
					"       Data operations like DELETE are not part of schema definitions\n"+
					"  Learn more: https://github.com/zakandrewking/lockplane#schema-definition-formats",
					tableName),
				Code: "dangerous_delete_all",
			})
		}

	case *pg_query.Node_AlterTableStmt:
		// ALTER TABLE ... DROP COLUMN
		alterStmt := node.AlterTableStmt
		tableName := extractRangeVarName(alterStmt.Relation)

		for _, cmd := range alterStmt.Cmds {
			if cmd.Node == nil {
				continue
			}

			if alterCmd, ok := cmd.Node.(*pg_query.Node_AlterTableCmd); ok {
				if alterCmd.AlterTableCmd.Subtype == pg_query.AlterTableType_AT_DropColumn {
					columnName := alterCmd.AlterTableCmd.Name

					issues = append(issues, ValidationIssue{
						File:     filePath,
						Line:     line,
						Column:   1,
						Severity: "error",
						Message: fmt.Sprintf("DROP COLUMN is not allowed in .lp.sql schema files\n"+
							"  Found: ALTER TABLE %s DROP COLUMN %s\n"+
							"  Why: .lp.sql files are declarative schema definitions (CREATE only)\n"+
							"       Destructive operations like DROP COLUMN are handled by Lockplane migrations\n"+
							"  To drop a column: Remove it from your table definition, then run 'lockplane apply'\n"+
							"  Learn more: https://github.com/zakandrewking/lockplane#schema-definition-formats",
							tableName, columnName),
						Code: "dangerous_drop_column",
					})
				}
			}
		}
	}

	return issues
}

// Helper functions to extract names from AST nodes

func extractObjectName(objects []*pg_query.Node) string {
	if len(objects) == 0 {
		return "unknown"
	}

	// Objects is a list of lists (for qualified names like schema.table)
	if listNode, ok := objects[0].Node.(*pg_query.Node_List); ok {
		names := []string{}
		for _, item := range listNode.List.Items {
			if strNode, ok := item.Node.(*pg_query.Node_String_); ok {
				names = append(names, strNode.String_.Sval)
			}
		}
		return strings.Join(names, ".")
	}

	return "unknown"
}

func extractRelationNames(relations []*pg_query.Node) []string {
	names := []string{}
	for _, rel := range relations {
		names = append(names, extractRelationName(rel))
	}
	return names
}

func extractRelationName(relation *pg_query.Node) string {
	if relation == nil {
		return "unknown"
	}

	if rangeVar, ok := relation.Node.(*pg_query.Node_RangeVar); ok {
		if rangeVar.RangeVar.Schemaname != "" {
			return rangeVar.RangeVar.Schemaname + "." + rangeVar.RangeVar.Relname
		}
		return rangeVar.RangeVar.Relname
	}

	return "unknown"
}

func extractRangeVarName(rangeVar *pg_query.RangeVar) string {
	if rangeVar == nil {
		return "unknown"
	}

	if rangeVar.Schemaname != "" {
		return rangeVar.Schemaname + "." + rangeVar.Relname
	}
	return rangeVar.Relname
}

func getCascadeWarning(cascade string) string {
	if cascade != "" {
		return "\n                  CASCADE will also drop all dependent objects (foreign keys, views, etc.)"
	}
	return ""
}

// detectNonDeclarativePatterns detects SQL patterns that shouldn't be in declarative schema files
func detectNonDeclarativePatterns(filePath string, stmt *pg_query.Node, line int) []ValidationIssue {
	var issues []ValidationIssue

	switch node := stmt.Node.(type) {
	case *pg_query.Node_CreateStmt:
		// CREATE TABLE with IF NOT EXISTS
		createStmt := node.CreateStmt
		if createStmt.IfNotExists {
			tableName := extractRangeVarName(createStmt.Relation)
			issues = append(issues, ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   1,
				Severity: "error",
				Message: fmt.Sprintf("IF NOT EXISTS should not be used in declarative schema files\n"+
					"  Found: CREATE TABLE IF NOT EXISTS %s\n"+
					"  Impact: Makes schema non-deterministic and harder to version control\n"+
					"  Recommendation: Remove IF NOT EXISTS - Lockplane manages existence checks\n"+
					"                  Use: CREATE TABLE %s",
					tableName, tableName),
				Code: "non_declarative_if_not_exists",
			})
		}

	case *pg_query.Node_IndexStmt:
		// CREATE INDEX with IF NOT EXISTS
		indexStmt := node.IndexStmt
		if indexStmt.IfNotExists {
			indexName := indexStmt.Idxname
			if indexName == "" {
				indexName = "unnamed_index"
			}
			issues = append(issues, ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   1,
				Severity: "error",
				Message: fmt.Sprintf("IF NOT EXISTS should not be used in declarative schema files\n"+
					"  Found: CREATE INDEX IF NOT EXISTS %s\n"+
					"  Impact: Makes schema non-deterministic and harder to version control\n"+
					"  Recommendation: Remove IF NOT EXISTS - Lockplane manages existence checks",
					indexName),
				Code: "non_declarative_if_not_exists",
			})
		}

	case *pg_query.Node_TransactionStmt:
		// BEGIN, COMMIT, ROLLBACK, etc.
		txnStmt := node.TransactionStmt
		var txnType string
		switch txnStmt.Kind {
		case pg_query.TransactionStmtKind_TRANS_STMT_BEGIN:
			txnType = "BEGIN"
		case pg_query.TransactionStmtKind_TRANS_STMT_COMMIT:
			txnType = "COMMIT"
		case pg_query.TransactionStmtKind_TRANS_STMT_ROLLBACK:
			txnType = "ROLLBACK"
		case pg_query.TransactionStmtKind_TRANS_STMT_START:
			txnType = "START TRANSACTION"
		default:
			txnType = "transaction control"
		}

		issues = append(issues, ValidationIssue{
			File:     filePath,
			Line:     line,
			Column:   1,
			Severity: "error",
			Message: fmt.Sprintf("Transaction control statements should not be in schema files\n"+
				"  Found: %s\n"+
				"  Impact: Schema files should be declarative definitions only\n"+
				"  Recommendation: Remove %s - Lockplane manages transactions automatically\n"+
				"                  Migration plans are executed in transactions by default",
				txnType, txnType),
			Code: "non_declarative_transaction",
		})

	case *pg_query.Node_ViewStmt:
		// CREATE OR REPLACE VIEW
		viewStmt := node.ViewStmt
		if viewStmt.Replace {
			viewName := extractRangeVarName(viewStmt.View)
			issues = append(issues, ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   1,
				Severity: "error",
				Message: fmt.Sprintf("CREATE OR REPLACE should not be used in declarative schema files\n"+
					"  Found: CREATE OR REPLACE VIEW %s\n"+
					"  Impact: Makes schema non-deterministic and harder to version control\n"+
					"  Recommendation: Use CREATE VIEW - Lockplane handles updates via DROP/CREATE\n"+
					"                  Or use: CREATE VIEW %s",
					viewName, viewName),
				Code: "non_declarative_or_replace",
			})
		}
	}

	return issues
}
