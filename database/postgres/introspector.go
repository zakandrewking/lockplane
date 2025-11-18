package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lockplane/lockplane/database"
)

// Introspector implements database.Introspector for PostgreSQL
type Introspector struct{}

// NewIntrospector creates a new PostgreSQL introspector
func NewIntrospector() *Introspector {
	return &Introspector{}
}

// IntrospectSchema reads the entire PostgreSQL database schema from current_schema()
func (i *Introspector) IntrospectSchema(ctx context.Context, db *sql.DB) (*database.Schema, error) {
	// Default to current_schema() for backwards compatibility
	return i.IntrospectSchemas(ctx, db, nil)
}

// IntrospectSchemas reads tables from multiple PostgreSQL schemas
// If schemas is nil or empty, uses current_schema()
func (i *Introspector) IntrospectSchemas(ctx context.Context, db *sql.DB, schemas []string) (*database.Schema, error) {
	schema := &database.Schema{
		Tables: make([]database.Table, 0),
	}

	// If no schemas specified, use current_schema()
	if len(schemas) == 0 {
		currentSchema, err := i.getCurrentSchema(ctx, db)
		if err != nil {
			return nil, fmt.Errorf("failed to get current schema: %w", err)
		}
		schemas = []string{currentSchema}
	}

	// Introspect each schema
	for _, schemaName := range schemas {
		tables, err := i.GetTablesInSchema(ctx, db, schemaName)
		if err != nil {
			return nil, fmt.Errorf("failed to get tables in schema %s: %w", schemaName, err)
		}

		for _, tableName := range tables {
			table := database.Table{
				Name:   tableName,
				Schema: schemaName,
			}

			columns, err := i.GetColumnsInSchema(ctx, db, schemaName, tableName)
			if err != nil {
				return nil, fmt.Errorf("failed to get columns for table %s.%s: %w", schemaName, tableName, err)
			}
			table.Columns = columns

			indexes, err := i.GetIndexesInSchema(ctx, db, schemaName, tableName)
			if err != nil {
				return nil, fmt.Errorf("failed to get indexes for table %s.%s: %w", schemaName, tableName, err)
			}
			table.Indexes = indexes

			foreignKeys, err := i.GetForeignKeysInSchema(ctx, db, schemaName, tableName)
			if err != nil {
				return nil, fmt.Errorf("failed to get foreign keys for table %s.%s: %w", schemaName, tableName, err)
			}
			table.ForeignKeys = foreignKeys

			// Get RLS status
			rlsEnabled, err := i.GetRLSEnabledInSchema(ctx, db, schemaName, tableName)
			if err != nil {
				return nil, fmt.Errorf("failed to get RLS status for table %s.%s: %w", schemaName, tableName, err)
			}
			table.RLSEnabled = rlsEnabled

			// Get RLS policies if RLS is enabled
			if rlsEnabled {
				policies, err := i.GetPoliciesInSchema(ctx, db, schemaName, tableName)
				if err != nil {
					return nil, fmt.Errorf("failed to get policies for table %s.%s: %w", schemaName, tableName, err)
				}
				table.Policies = policies
			}

			schema.Tables = append(schema.Tables, table)
		}
	}

	schema.Dialect = database.DialectPostgres
	return schema, nil
}

// getCurrentSchema gets the current PostgreSQL schema
func (i *Introspector) getCurrentSchema(ctx context.Context, db *sql.DB) (string, error) {
	var schemaName string
	err := db.QueryRowContext(ctx, "SELECT current_schema()").Scan(&schemaName)
	if err != nil {
		return "", err
	}
	return schemaName, nil
}

// GetTables returns all table names in the current PostgreSQL schema
func (i *Introspector) GetTables(ctx context.Context, db *sql.DB) ([]string, error) {
	currentSchema, err := i.getCurrentSchema(ctx, db)
	if err != nil {
		return nil, err
	}
	return i.GetTablesInSchema(ctx, db, currentSchema)
}

// GetTablesInSchema returns all table names in a specific PostgreSQL schema
func (i *Introspector) GetTablesInSchema(ctx context.Context, db *sql.DB, schemaName string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables in schema %s: %w", schemaName, err)
	}
	defer func() { _ = rows.Close() }()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	return tableNames, nil
}

// GetColumns returns all columns for a given PostgreSQL table in current_schema()
func (i *Introspector) GetColumns(ctx context.Context, db *sql.DB, tableName string) ([]database.Column, error) {
	currentSchema, err := i.getCurrentSchema(ctx, db)
	if err != nil {
		return nil, err
	}
	return i.GetColumnsInSchema(ctx, db, currentSchema, tableName)
}

// GetColumnsInSchema returns all columns for a given PostgreSQL table in a specific schema
func (i *Introspector) GetColumnsInSchema(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]database.Column, error) {
	query := `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			COALESCE(
				(SELECT true
				 FROM information_schema.table_constraints tc
				 JOIN information_schema.key_column_usage kcu
				   ON tc.constraint_name = kcu.constraint_name
				   AND tc.table_schema = kcu.table_schema
				 WHERE tc.table_name = c.table_name
				   AND tc.table_schema = c.table_schema
				   AND tc.constraint_type = 'PRIMARY KEY'
				   AND kcu.column_name = c.column_name),
				false
			) as is_primary_key
		FROM information_schema.columns c
		WHERE c.table_schema = $1
		  AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []database.Column
	for rows.Next() {
		var col database.Column
		var nullable string
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &col.IsPrimaryKey); err != nil {
			return nil, err
		}

		col.Type = strings.TrimSpace(col.Type)

		// Detect SERIAL/BIGSERIAL pseudo-types
		// PostgreSQL converts BIGSERIAL to BIGINT with nextval() default
		// and SERIAL to INTEGER with nextval() default
		actualType := col.Type
		isSerial := false
		if defaultVal.Valid && isSerialDefault(defaultVal.String) {
			if strings.EqualFold(col.Type, "bigint") {
				actualType = "bigserial"
				isSerial = true
			} else if strings.EqualFold(col.Type, "integer") {
				actualType = "serial"
				isSerial = true
			}
		}

		col.Type = actualType
		col.TypeMetadata = &database.TypeMetadata{
			Logical: strings.ToLower(actualType),
			Raw:     actualType,
			Dialect: database.DialectPostgres,
		}

		col.Nullable = nullable == "YES"

		// For SERIAL/BIGSERIAL, the default is implicit, so don't store it
		// For other columns, normalize the default value
		if isSerial {
			col.Default = nil
			col.DefaultMetadata = nil
		} else if defaultVal.Valid {
			// Normalize default values by removing unnecessary type casts
			normalized := normalizeDefault(defaultVal.String)
			col.Default = &normalized
			col.DefaultMetadata = &database.DefaultMetadata{
				Raw:     normalized,
				Dialect: database.DialectPostgres,
			}
		} else {
			col.DefaultMetadata = nil
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// GetIndexes returns all indexes for a given PostgreSQL table in current_schema()
// Excludes indexes that are automatically created by PRIMARY KEY or UNIQUE constraints
func (i *Introspector) GetIndexes(ctx context.Context, db *sql.DB, tableName string) ([]database.Index, error) {
	currentSchema, err := i.getCurrentSchema(ctx, db)
	if err != nil {
		return nil, err
	}
	return i.GetIndexesInSchema(ctx, db, currentSchema, tableName)
}

// GetIndexesInSchema returns all indexes for a given PostgreSQL table in a specific schema
// Excludes indexes that are automatically created by PRIMARY KEY or UNIQUE constraints
func (i *Introspector) GetIndexesInSchema(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]database.Index, error) {
	query := `
		SELECT
			i.indexname,
			i.indexdef,
			ix.indisunique
		FROM pg_indexes i
		JOIN pg_class c ON c.relname = i.tablename
		JOIN pg_index ix ON ix.indexrelid = (
			SELECT oid FROM pg_class WHERE relname = i.indexname AND relnamespace = (
				SELECT oid FROM pg_namespace WHERE nspname = $1
			)
		)
		WHERE i.schemaname = $1
		  AND i.tablename = $2
		  AND ix.indisprimary = false
		  AND NOT EXISTS (
			SELECT 1
			FROM pg_constraint con
			WHERE con.conindid = ix.indexrelid
			  AND con.contype IN ('p', 'u')
		  )
		ORDER BY i.indexname
	`

	rows, err := db.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		// Add context about which table failed and the query being run
		return nil, fmt.Errorf("query failed for table %s.%s: %w\nQuery: %s", schemaName, tableName, err, query)
	}
	defer func() { _ = rows.Close() }()

	var indexes []database.Index
	for rows.Next() {
		var idx database.Index
		var indexDef string

		if err := rows.Scan(&idx.Name, &indexDef, &idx.Unique); err != nil {
			return nil, err
		}

		// TODO: Parse indexDef to extract column names properly
		// For now, just storing the index name and unique flag
		idx.Columns = []string{} // Simplified for MVP

		indexes = append(indexes, idx)
	}

	return indexes, nil
}

// GetForeignKeys returns all foreign keys for a given PostgreSQL table in current_schema()
func (i *Introspector) GetForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]database.ForeignKey, error) {
	currentSchema, err := i.getCurrentSchema(ctx, db)
	if err != nil {
		return nil, err
	}
	return i.GetForeignKeysInSchema(ctx, db, currentSchema, tableName)
}

// GetForeignKeysInSchema returns all foreign keys for a given PostgreSQL table in a specific schema
func (i *Introspector) GetForeignKeysInSchema(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]database.ForeignKey, error) {
	query := `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name,
			rc.update_rule,
			rc.delete_rule
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints AS rc
			ON rc.constraint_name = tc.constraint_name
			AND rc.constraint_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1
			AND tc.table_name = $2
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	// Group by constraint name to handle multi-column foreign keys
	fkMap := make(map[string]*database.ForeignKey)
	var fkNames []string

	for rows.Next() {
		var constraintName, columnName, foreignTableName, foreignColumnName string
		var updateRule, deleteRule string

		if err := rows.Scan(&constraintName, &columnName, &foreignTableName, &foreignColumnName, &updateRule, &deleteRule); err != nil {
			return nil, err
		}

		if _, exists := fkMap[constraintName]; !exists {
			fk := &database.ForeignKey{
				Name:              constraintName,
				Columns:           []string{},
				ReferencedTable:   foreignTableName,
				ReferencedColumns: []string{},
			}

			// Convert SQL standard actions to our format
			if updateRule != "NO ACTION" {
				fk.OnUpdate = &updateRule
			}
			if deleteRule != "NO ACTION" {
				fk.OnDelete = &deleteRule
			}

			fkMap[constraintName] = fk
			fkNames = append(fkNames, constraintName)
		}

		fkMap[constraintName].Columns = append(fkMap[constraintName].Columns, columnName)
		fkMap[constraintName].ReferencedColumns = append(fkMap[constraintName].ReferencedColumns, foreignColumnName)
	}

	// Convert map to slice in consistent order
	var foreignKeys []database.ForeignKey
	for _, name := range fkNames {
		foreignKeys = append(foreignKeys, *fkMap[name])
	}

	return foreignKeys, nil
}

// isSerialDefault checks if a default value is from a sequence (indicating SERIAL/BIGSERIAL)
func isSerialDefault(defaultVal string) bool {
	// SERIAL/BIGSERIAL columns have defaults like:
	// - nextval('tablename_columnname_seq'::regclass)
	// - nextval('sequence_name'::regclass)
	return strings.HasPrefix(defaultVal, "nextval(") && strings.Contains(defaultVal, "_seq")
}

// normalizeDefault normalizes PostgreSQL default values for comparison
// Removes type casts that are redundant (e.g., '{}'::jsonb -> '{}')
func normalizeDefault(defaultVal string) string {
	// Remove trailing type casts like ::jsonb, ::text, etc.
	// Pattern: anything::type at the end
	if idx := strings.LastIndex(defaultVal, "::"); idx > 0 {
		// Check if this is a type cast (not part of a string literal)
		beforeCast := defaultVal[:idx]
		// Simple heuristic: if it's balanced quotes, it's likely a cast
		if strings.Count(beforeCast, "'")%2 == 0 {
			return beforeCast
		}
	}
	return defaultVal
}

// GetRLSEnabled checks if Row Level Security is enabled for a table in current_schema()
func (i *Introspector) GetRLSEnabled(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	currentSchema, err := i.getCurrentSchema(ctx, db)
	if err != nil {
		return false, err
	}
	return i.GetRLSEnabledInSchema(ctx, db, currentSchema, tableName)
}

// GetRLSEnabledInSchema checks if Row Level Security is enabled for a table in a specific schema
func (i *Introspector) GetRLSEnabledInSchema(ctx context.Context, db *sql.DB, schemaName, tableName string) (bool, error) {
	query := `
		SELECT relrowsecurity
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1
		  AND n.nspname = $2
		  AND c.relkind = 'r'
	`

	var rlsEnabled bool
	err := db.QueryRowContext(ctx, query, tableName, schemaName).Scan(&rlsEnabled)
	if err != nil {
		// Table might not exist or query failed
		return false, err
	}

	return rlsEnabled, nil
}

// GetPolicies returns all RLS policies for a table in current_schema()
func (i *Introspector) GetPolicies(ctx context.Context, db *sql.DB, tableName string) ([]database.Policy, error) {
	currentSchema, err := i.getCurrentSchema(ctx, db)
	if err != nil {
		return nil, err
	}
	return i.GetPoliciesInSchema(ctx, db, currentSchema, tableName)
}

// GetPoliciesInSchema returns all RLS policies for a table in a specific schema
func (i *Introspector) GetPoliciesInSchema(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]database.Policy, error) {
	query := `
		SELECT
			pol.polname AS policy_name,
			CASE pol.polcmd
				WHEN 'r' THEN 'SELECT'
				WHEN 'a' THEN 'INSERT'
				WHEN 'w' THEN 'UPDATE'
				WHEN 'd' THEN 'DELETE'
				WHEN '*' THEN 'ALL'
			END AS command,
			pol.polpermissive AS permissive,
			ARRAY(
				SELECT pg_catalog.quote_ident(rolname)
				FROM pg_catalog.pg_roles
				WHERE oid = ANY(pol.polroles)
			) AS roles,
			pg_catalog.pg_get_expr(pol.polqual, pol.polrelid) AS using_expr,
			pg_catalog.pg_get_expr(pol.polwithcheck, pol.polrelid) AS with_check_expr
		FROM pg_catalog.pg_policy pol
		JOIN pg_catalog.pg_class c ON c.oid = pol.polrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relname = $2
		ORDER BY pol.polname
	`

	rows, err := db.QueryContext(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies for table %s.%s: %w", schemaName, tableName, err)
	}
	defer func() { _ = rows.Close() }()

	var policies []database.Policy
	for rows.Next() {
		var policy database.Policy
		var roles []string
		var usingExpr, withCheckExpr sql.NullString

		if err := rows.Scan(&policy.Name, &policy.Command, &policy.Permissive, &roles, &usingExpr, &withCheckExpr); err != nil {
			return nil, fmt.Errorf("failed to scan policy row: %w", err)
		}

		policy.Roles = roles

		if usingExpr.Valid {
			policy.Using = &usingExpr.String
		}

		if withCheckExpr.Valid {
			policy.WithCheck = &withCheckExpr.String
		}

		policies = append(policies, policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating policy rows: %w", err)
	}

	return policies, nil
}
