// Package parser provides SQL DDL parsing utilities for lockplane.
//
// This package uses pg_query to parse PostgreSQL DDL statements and extract
// schema information including tables, columns, indexes, and constraints.
package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lockplane/lockplane/database"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// SQL parsing utilities for extracting identifiers from SQL statements
// These are simplified parsers that work for the SQL we generate

// extractTableNameFromCreate extracts table name from CREATE TABLE statement
func ExtractTableNameFromCreate(sql string) (string, error) {
	// Pattern: CREATE TABLE <name> ...
	re := regexp.MustCompile(`CREATE\s+TABLE\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract table name from: %s", sql)
	}
	return matches[1], nil
}

// extractTableNameFromDrop extracts table name from DROP TABLE statement
func ExtractTableNameFromDrop(sql string) (string, error) {
	// Pattern: DROP TABLE <name> [CASCADE]
	re := regexp.MustCompile(`DROP\s+TABLE\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract table name from: %s", sql)
	}
	return matches[1], nil
}

// extractTableAndColumnFromAddColumn extracts table and column name from ALTER TABLE ADD COLUMN
func ExtractTableAndColumnFromAddColumn(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ADD COLUMN <column> ...
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromDropColumn extracts table and column name from ALTER TABLE DROP COLUMN
func ExtractTableAndColumnFromDropColumn(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> DROP COLUMN <column>
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+DROP\s+COLUMN\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromAlterType extracts table and column from ALTER COLUMN TYPE
func ExtractTableAndColumnFromAlterType(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ALTER COLUMN <column> TYPE <type>
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+TYPE`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromAlterNotNull extracts table and column from ALTER COLUMN SET/DROP NOT NULL
func ExtractTableAndColumnFromAlterNotNull(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ALTER COLUMN <column> SET/DROP NOT NULL
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+(SET|DROP)\s+NOT\s+NULL`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromSetDefault extracts table and column from SET DEFAULT
func ExtractTableAndColumnFromSetDefault(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ALTER COLUMN <column> SET DEFAULT ...
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+SET\s+DEFAULT`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndColumnFromDropDefault extracts table and column from DROP DEFAULT
func ExtractTableAndColumnFromDropDefault(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ALTER COLUMN <column> DROP DEFAULT
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+DROP\s+DEFAULT`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and column from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractIndexNameFromCreate extracts index name from CREATE INDEX
func ExtractIndexNameFromCreate(sql string) (string, error) {
	// Pattern: CREATE [UNIQUE] INDEX <name> ON ...
	re := regexp.MustCompile(`CREATE\s+(UNIQUE\s+)?INDEX\s+(\w+)\s+ON`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", fmt.Errorf("could not extract index name from: %s", sql)
	}
	return matches[2], nil
}

// extractIndexNameFromDrop extracts index name from DROP INDEX
func ExtractIndexNameFromDrop(sql string) (string, error) {
	// Pattern: DROP INDEX <name>
	re := regexp.MustCompile(`DROP\s+INDEX\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract index name from: %s", sql)
	}
	return matches[1], nil
}

// extractTableAndConstraintFromAddConstraint extracts table and constraint name from ADD CONSTRAINT
func ExtractTableAndConstraintFromAddConstraint(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> ADD CONSTRAINT <constraint> ...
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+ADD\s+CONSTRAINT\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and constraint from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// extractTableAndConstraintFromDropConstraint extracts table and constraint name from DROP CONSTRAINT
func ExtractTableAndConstraintFromDropConstraint(sql string) (string, string, error) {
	// Pattern: ALTER TABLE <table> DROP CONSTRAINT <constraint>
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)\s+DROP\s+CONSTRAINT\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("could not extract table and constraint from: %s", sql)
	}
	return matches[1], matches[2], nil
}

// ContainsSQL is a helper to check if SQL contains a substring (case-insensitive)
func ContainsSQL(sql, substr string) bool {
	return strings.Contains(strings.ToUpper(sql), strings.ToUpper(substr))
}

// findTable locates a table by name within the schema
func findTable(schema *database.Schema, name string) *database.Table {
	for i := range schema.Tables {
		if schema.Tables[i].Name == name {
			return &schema.Tables[i]
		}
	}
	return nil
}

// findColumnIndex finds a column index within a table by name
func findColumnIndex(table *database.Table, columnName string) int {
	for i := range table.Columns {
		if table.Columns[i].Name == columnName {
			return i
		}
	}
	return -1
}

// removeIndexByName removes an index from a table by name
func removeIndexByName(table *database.Table, name string) bool {
	for i := range table.Indexes {
		if table.Indexes[i].Name == name {
			table.Indexes = append(table.Indexes[:i], table.Indexes[i+1:]...)
			return true
		}
	}
	return false
}

// removeForeignKeyByName removes a foreign key from a table by name
func removeForeignKeyByName(table *database.Table, name string) bool {
	for i := range table.ForeignKeys {
		if table.ForeignKeys[i].Name == name {
			table.ForeignKeys = append(table.ForeignKeys[:i], table.ForeignKeys[i+1:]...)
			return true
		}
	}
	return false
}

// dropPrimaryKey clears the primary key flags on all columns
func dropPrimaryKey(table *database.Table) bool {
	var hadPrimaryKey bool
	for i := range table.Columns {
		if table.Columns[i].IsPrimaryKey {
			table.Columns[i].IsPrimaryKey = false
			hadPrimaryKey = true
		}
	}
	return hadPrimaryKey
}

// ParseSQLSchema parses SQL DDL assuming PostgreSQL dialect.
func ParseSQLSchema(sql string) (*database.Schema, error) {
	return ParseSQLSchemaWithDialect(sql, database.DialectPostgres)
}

// ParseSQLSchemaWithDialect parses SQL DDL for the requested dialect.
func ParseSQLSchemaWithDialect(sql string, dialect database.Dialect) (*database.Schema, error) {
	switch dialect {
	case database.DialectSQLite:
		return parseSQLiteSQLSchema(sql)
	case database.DialectPostgres, database.DialectUnknown:
		return parsePostgresSQLSchema(sql)
	default:
		return parsePostgresSQLSchema(sql)
	}
}

// parsePostgresSQLSchema parses SQL DDL via pg_query for PostgreSQL schemas.
func parsePostgresSQLSchema(sql string) (*database.Schema, error) {
	// Parse the SQL
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %w", err)
	}

	schema := &database.Schema{
		Tables:  []database.Table{},
		Dialect: database.DialectPostgres,
	}

	// Walk the parse tree
	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}

		switch node := stmt.Stmt.Node.(type) {
		case *pg_query.Node_CreateStmt:
			table, err := parseCreateTable(node.CreateStmt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse CREATE TABLE: %w", err)
			}
			schema.Tables = append(schema.Tables, *table)

		case *pg_query.Node_IndexStmt:
			// Handle CREATE INDEX separately (will add to existing table)
			err := parseCreateIndex(schema, node.IndexStmt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse CREATE INDEX: %w", err)
			}

		case *pg_query.Node_AlterTableStmt:
			// ALTER TABLE warnings are now handled by the validation layer (cmd/plan.go)
			// which provides structured diagnostics with file/line/column info
			err := parseAlterTable(schema, node.AlterTableStmt)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ALTER TABLE: %w", err)
			}

			// We can add more statement types later (ALTER TABLE, etc.)
		}
	}

	return schema, nil
}

// parseCreateTable converts a CreateStmt AST node to a Table
func parseCreateTable(stmt *pg_query.CreateStmt) (*database.Table, error) {
	if stmt.Relation == nil {
		return nil, fmt.Errorf("CREATE TABLE missing relation")
	}

	table := &database.Table{
		Name:        stmt.Relation.Relname,
		Columns:     []database.Column{},
		Indexes:     []database.Index{},
		ForeignKeys: []database.ForeignKey{},
	}

	// Parse columns and constraints
	for _, elt := range stmt.TableElts {
		if elt.Node == nil {
			continue
		}

		switch node := elt.Node.(type) {
		case *pg_query.Node_ColumnDef:
			col, err := parseColumnDef(node.ColumnDef)
			if err != nil {
				return nil, err
			}
			table.Columns = append(table.Columns, *col)

		case *pg_query.Node_Constraint:
			err := parseTableConstraint(table, node.Constraint)
			if err != nil {
				return nil, err
			}
		}
	}

	return table, nil
}

// parseColumnDef converts a ColumnDef AST node to a Column
func parseColumnDef(colDef *pg_query.ColumnDef) (*database.Column, error) {
	if colDef.Colname == "" {
		return nil, fmt.Errorf("column missing name")
	}

	col := &database.Column{
		Name:         colDef.Colname,
		Nullable:     true, // Default to nullable unless NOT NULL is specified
		IsPrimaryKey: false,
	}

	// Parse type
	if colDef.TypeName != nil {
		colType, meta := formatTypeName(colDef.TypeName)
		col.Type = colType
		col.TypeMetadata = meta
	}

	// Parse constraints (NOT NULL, DEFAULT, PRIMARY KEY, etc.)
	for _, constraint := range colDef.Constraints {
		if constraint.Node == nil {
			continue
		}

		if cons, ok := constraint.Node.(*pg_query.Node_Constraint); ok {
			parseColumnConstraint(col, cons.Constraint)
		}
	}

	return col, nil
}

// formatTypeName converts TypeName AST to a string representation with metadata.
func formatTypeName(typeName *pg_query.TypeName) (string, *database.TypeMetadata) {
	if len(typeName.Names) == 0 {
		return "", nil
	}

	// Get the type name (last element in Names)
	var parts []string
	for _, name := range typeName.Names {
		if nameNode, ok := name.Node.(*pg_query.Node_String_); ok {
			parts = append(parts, nameNode.String_.Sval)
		}
	}

	rawBase := strings.Join(parts, ".")
	typeStr := rawBase

	if len(parts) > 1 && parts[0] == "pg_catalog" {
		typeStr = parts[len(parts)-1]
	}

	// Normalize PostgreSQL internal types to standard SQL types
	typeStr = normalizePostgreSQLType(typeStr)

	// Add type modifiers (e.g., VARCHAR(255))
	if len(typeName.Typmods) > 0 {
		var mods []string
		for _, mod := range typeName.Typmods {
			if constNode, ok := mod.Node.(*pg_query.Node_AConst); ok {
				if ival := constNode.AConst.GetIval(); ival != nil {
					mods = append(mods, fmt.Sprintf("%d", ival.Ival))
				}
			}
		}
		if len(mods) > 0 {
			modStr := strings.Join(mods, ",")
			typeStr = fmt.Sprintf("%s(%s)", typeStr, modStr)
			rawBase = fmt.Sprintf("%s(%s)", rawBase, modStr)
		}
	}

	// Add array notation if needed
	if len(typeName.ArrayBounds) > 0 {
		typeStr += "[]"
		rawBase += "[]"
	}

	meta := &database.TypeMetadata{
		Logical: typeStr,
		Raw:     rawBase,
		Dialect: database.DialectPostgres,
	}

	return typeStr, meta
}

// normalizePostgreSQLType converts PostgreSQL internal type names to standard SQL types
// This is necessary because we use pg_query (PostgreSQL parser) for all SQL parsing,
// and it normalizes types to PostgreSQL internal names like "int4", "int8", "bool", etc.
func normalizePostgreSQLType(pgType string) string {
	// Map PostgreSQL internal types to standard SQL types
	typeMap := map[string]string{
		// Integer types
		"int2":    "smallint",
		"int4":    "integer",
		"int8":    "bigint",
		"serial":  "serial",
		"serial2": "smallserial",
		"serial4": "serial",
		"serial8": "bigserial",

		// Boolean
		"bool": "boolean",

		// Character types
		"varchar": "varchar",
		"bpchar":  "char",

		// Floating point
		"float4": "real",
		"float8": "double precision",

		// Timestamp types
		"timestamptz": "timestamp with time zone",
		"timetz":      "time with time zone",

		// Text (keep as-is, but explicitly map)
		"text": "text",

		// Numeric
		"numeric": "numeric",
		"decimal": "decimal",
	}

	if normalized, ok := typeMap[strings.ToLower(pgType)]; ok {
		return normalized
	}

	return pgType
}

// parseColumnConstraint applies a column-level constraint to a Column
func parseColumnConstraint(col *database.Column, constraint *pg_query.Constraint) {
	switch constraint.Contype {
	case pg_query.ConstrType_CONSTR_NOTNULL:
		col.Nullable = false

	case pg_query.ConstrType_CONSTR_NULL:
		col.Nullable = true

	case pg_query.ConstrType_CONSTR_DEFAULT:
		if constraint.RawExpr != nil {
			// Format the default expression
			defaultStr := formatExpr(constraint.RawExpr)
			col.Default = &defaultStr
			col.DefaultMetadata = &database.DefaultMetadata{
				Raw:     defaultStr,
				Dialect: database.DialectPostgres,
			}
		}

	case pg_query.ConstrType_CONSTR_PRIMARY:
		col.IsPrimaryKey = true
		col.Nullable = false // PRIMARY KEY implies NOT NULL
	}
}

// parseTableConstraint applies a table-level constraint
func parseTableConstraint(table *database.Table, constraint *pg_query.Constraint) error {
	switch constraint.Contype {
	case pg_query.ConstrType_CONSTR_PRIMARY:
		// Mark columns as primary key
		for _, key := range constraint.Keys {
			if keyNode, ok := key.Node.(*pg_query.Node_String_); ok {
				colName := keyNode.String_.Sval
				for i := range table.Columns {
					if table.Columns[i].Name == colName {
						table.Columns[i].IsPrimaryKey = true
						table.Columns[i].Nullable = false
					}
				}
			}
		}

	case pg_query.ConstrType_CONSTR_UNIQUE:
		// Create a unique index
		idx := database.Index{
			Name:    getConstraintName(constraint, table.Name, "unique"),
			Unique:  true,
			Columns: []string{},
		}
		for _, key := range constraint.Keys {
			if keyNode, ok := key.Node.(*pg_query.Node_String_); ok {
				idx.Columns = append(idx.Columns, keyNode.String_.Sval)
			}
		}
		if len(idx.Columns) > 0 {
			table.Indexes = append(table.Indexes, idx)
		}

	case pg_query.ConstrType_CONSTR_FOREIGN:
		// Create foreign key
		fk := database.ForeignKey{
			Name:              getConstraintName(constraint, table.Name, "fk"),
			Columns:           []string{},
			ReferencedColumns: []string{},
		}

		// Source columns
		for _, key := range constraint.FkAttrs {
			if keyNode, ok := key.Node.(*pg_query.Node_String_); ok {
				fk.Columns = append(fk.Columns, keyNode.String_.Sval)
			}
		}

		// Referenced table
		if constraint.Pktable != nil && constraint.Pktable.Relname != "" {
			fk.ReferencedTable = constraint.Pktable.Relname
		}

		// Referenced columns
		for _, key := range constraint.PkAttrs {
			if keyNode, ok := key.Node.(*pg_query.Node_String_); ok {
				fk.ReferencedColumns = append(fk.ReferencedColumns, keyNode.String_.Sval)
			}
		}

		// ON DELETE/UPDATE actions
		if constraint.FkDelAction != "" {
			action := formatForeignKeyAction(constraint.FkDelAction)
			fk.OnDelete = &action
		}
		if constraint.FkUpdAction != "" {
			action := formatForeignKeyAction(constraint.FkUpdAction)
			fk.OnUpdate = &action
		}

		if len(fk.Columns) > 0 && fk.ReferencedTable != "" {
			table.ForeignKeys = append(table.ForeignKeys, fk)
		}
	}

	return nil
}

// parseAlterTable applies ALTER TABLE commands to the schema
func parseAlterTable(schema *database.Schema, stmt *pg_query.AlterTableStmt) error {
	if stmt.Relation == nil || stmt.Relation.Relname == "" {
		return fmt.Errorf("ALTER TABLE missing relation")
	}

	table := findTable(schema, stmt.Relation.Relname)
	if table == nil {
		return fmt.Errorf("ALTER TABLE references unknown table: %s", stmt.Relation.Relname)
	}

	for _, cmdNode := range stmt.Cmds {
		if cmdNode == nil {
			continue
		}

		alterCmd, ok := cmdNode.Node.(*pg_query.Node_AlterTableCmd)
		if !ok || alterCmd.AlterTableCmd == nil {
			continue
		}

		if err := applyAlterTableCmd(table, alterCmd.AlterTableCmd); err != nil {
			return err
		}
	}

	return nil
}

// applyAlterTableCmd mutates a table based on a single ALTER TABLE command
func applyAlterTableCmd(table *database.Table, cmd *pg_query.AlterTableCmd) error {
	if cmd == nil {
		return nil
	}

	switch cmd.Subtype {
	case pg_query.AlterTableType_AT_AddColumn:
		def := cmd.GetDef()
		if def == nil {
			return fmt.Errorf("ALTER TABLE %s ADD COLUMN missing definition", table.Name)
		}
		colDef := def.GetColumnDef()
		if colDef == nil {
			return fmt.Errorf("ALTER TABLE %s ADD COLUMN missing definition", table.Name)
		}
		col, err := parseColumnDef(colDef)
		if err != nil {
			return err
		}
		table.Columns = append(table.Columns, *col)

	case pg_query.AlterTableType_AT_DropColumn:
		if cmd.Name == "" {
			return fmt.Errorf("ALTER TABLE %s DROP COLUMN missing column name", table.Name)
		}
		idx := findColumnIndex(table, cmd.Name)
		if idx == -1 {
			return fmt.Errorf("ALTER TABLE %s DROP COLUMN unknown column: %s", table.Name, cmd.Name)
		}
		table.Columns = append(table.Columns[:idx], table.Columns[idx+1:]...)

	case pg_query.AlterTableType_AT_SetNotNull:
		if cmd.Name == "" {
			return fmt.Errorf("ALTER TABLE %s SET NOT NULL missing column name", table.Name)
		}
		idx := findColumnIndex(table, cmd.Name)
		if idx == -1 {
			return fmt.Errorf("ALTER TABLE %s SET NOT NULL unknown column: %s", table.Name, cmd.Name)
		}
		table.Columns[idx].Nullable = false

	case pg_query.AlterTableType_AT_DropNotNull:
		if cmd.Name == "" {
			return fmt.Errorf("ALTER TABLE %s DROP NOT NULL missing column name", table.Name)
		}
		idx := findColumnIndex(table, cmd.Name)
		if idx == -1 {
			return fmt.Errorf("ALTER TABLE %s DROP NOT NULL unknown column: %s", table.Name, cmd.Name)
		}
		table.Columns[idx].Nullable = true

	case pg_query.AlterTableType_AT_ColumnDefault:
		if cmd.Name == "" {
			return fmt.Errorf("ALTER TABLE %s ALTER COLUMN DEFAULT missing column name", table.Name)
		}
		idx := findColumnIndex(table, cmd.Name)
		if idx == -1 {
			return fmt.Errorf("ALTER TABLE %s ALTER COLUMN DEFAULT unknown column: %s", table.Name, cmd.Name)
		}
		if cmd.Def != nil {
			defaultStr := formatExpr(cmd.Def)
			table.Columns[idx].Default = &defaultStr
			table.Columns[idx].DefaultMetadata = &database.DefaultMetadata{
				Raw:     defaultStr,
				Dialect: database.DialectPostgres,
			}
		} else {
			table.Columns[idx].Default = nil
			table.Columns[idx].DefaultMetadata = nil
		}

	case pg_query.AlterTableType_AT_AlterColumnType:
		if cmd.Name == "" {
			return fmt.Errorf("ALTER TABLE %s ALTER COLUMN TYPE missing column name", table.Name)
		}
		idx := findColumnIndex(table, cmd.Name)
		if idx == -1 {
			return fmt.Errorf("ALTER TABLE %s ALTER COLUMN TYPE unknown column: %s", table.Name, cmd.Name)
		}
		def := cmd.GetDef()
		if def == nil {
			return fmt.Errorf("ALTER TABLE %s ALTER COLUMN %s missing type definition", table.Name, cmd.Name)
		}
		colDef := def.GetColumnDef()
		if colDef == nil || colDef.TypeName == nil {
			return fmt.Errorf("ALTER TABLE %s ALTER COLUMN %s missing type definition", table.Name, cmd.Name)
		}
		colType, meta := formatTypeName(colDef.TypeName)
		table.Columns[idx].Type = colType
		table.Columns[idx].TypeMetadata = meta

	case pg_query.AlterTableType_AT_AddConstraint:
		constraint := cmd.GetDef().GetConstraint()
		if constraint == nil {
			return fmt.Errorf("ALTER TABLE %s ADD CONSTRAINT missing definition", table.Name)
		}
		if err := parseTableConstraint(table, constraint); err != nil {
			return err
		}

	case pg_query.AlterTableType_AT_DropConstraint:
		if cmd.Name == "" {
			return fmt.Errorf("ALTER TABLE %s DROP CONSTRAINT missing constraint name", table.Name)
		}
		if removeIndexByName(table, cmd.Name) {
			return nil
		}
		if removeForeignKeyByName(table, cmd.Name) {
			return nil
		}
		if dropPrimaryKey(table) {
			return nil
		}
		return fmt.Errorf("ALTER TABLE %s DROP CONSTRAINT unsupported constraint: %s", table.Name, cmd.Name)

	case pg_query.AlterTableType_AT_EnableRowSecurity:
		table.RLSEnabled = true

	case pg_query.AlterTableType_AT_DisableRowSecurity:
		table.RLSEnabled = false

	default:
		return fmt.Errorf("ALTER TABLE %s unsupported command subtype: %s", table.Name, cmd.Subtype.String())
	}

	return nil
}

// parseCreateIndex handles CREATE INDEX statements
func parseCreateIndex(schema *database.Schema, stmt *pg_query.IndexStmt) error {
	if stmt.Relation == nil || stmt.Relation.Relname == "" {
		return fmt.Errorf("CREATE INDEX missing table name")
	}

	tableName := stmt.Relation.Relname

	// Find the table
	var targetTable *database.Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == tableName {
			targetTable = &schema.Tables[i]
			break
		}
	}

	if targetTable == nil {
		return fmt.Errorf("CREATE INDEX references unknown table: %s", tableName)
	}

	// Create index
	idx := database.Index{
		Name:    stmt.Idxname,
		Unique:  stmt.Unique,
		Columns: []string{},
	}

	// Extract column names
	for _, elem := range stmt.IndexParams {
		if elem.Node == nil {
			continue
		}
		indexElem, ok := elem.Node.(*pg_query.Node_IndexElem)
		if !ok || indexElem.IndexElem == nil {
			continue
		}

		colName := extractIndexColumnName(indexElem.IndexElem)
		if colName != "" {
			idx.Columns = append(idx.Columns, colName)
		}
	}

	if len(idx.Columns) > 0 {
		targetTable.Indexes = append(targetTable.Indexes, idx)
	}

	return nil
}

func extractIndexColumnName(elem *pg_query.IndexElem) string {
	if elem == nil {
		return ""
	}

	if elem.Name != "" {
		return elem.Name
	}

	if elem.Indexcolname != "" {
		return elem.Indexcolname
	}

	if expr := elem.Expr; expr != nil {
		if colRefNode, ok := expr.Node.(*pg_query.Node_ColumnRef); ok {
			if name := extractColumnRefName(colRefNode.ColumnRef); name != "" {
				return name
			}
		}
	}

	return ""
}

func extractColumnRefName(colRef *pg_query.ColumnRef) string {
	if colRef == nil {
		return ""
	}

	var last string
	for _, field := range colRef.Fields {
		if field == nil || field.Node == nil {
			continue
		}

		switch node := field.Node.(type) {
		case *pg_query.Node_String_:
			last = node.String_.Sval
		}
	}

	return last
}

// getConstraintName returns the constraint name or generates one
func getConstraintName(constraint *pg_query.Constraint, tableName, prefix string) string {
	if constraint.Conname != "" {
		return constraint.Conname
	}
	// Generate a name
	return fmt.Sprintf("%s_%s", tableName, prefix)
}

// formatForeignKeyAction converts foreign key action code to string
func formatForeignKeyAction(action string) string {
	if action == "" {
		return "NO ACTION"
	}
	// In pg_query v6, action might be a single character string or the full action name
	if len(action) == 1 {
		switch action[0] {
		case 'a': // FKCONSTR_ACTION_NOACTION
			return "NO ACTION"
		case 'r': // FKCONSTR_ACTION_RESTRICT
			return "RESTRICT"
		case 'c': // FKCONSTR_ACTION_CASCADE
			return "CASCADE"
		case 'n': // FKCONSTR_ACTION_SETNULL
			return "SET NULL"
		case 'd': // FKCONSTR_ACTION_SETDEFAULT
			return "SET DEFAULT"
		}
	}
	// If it's already the full action name, return as-is
	return action
}

// formatExpr converts an expression AST to string
func formatExpr(node *pg_query.Node) string {
	if node == nil {
		return ""
	}

	switch expr := node.Node.(type) {
	case *pg_query.Node_AConst:
		// Check different types of constants
		if ival := expr.AConst.GetIval(); ival != nil {
			return fmt.Sprintf("%d", ival.Ival)
		}
		if fval := expr.AConst.GetFval(); fval != nil {
			return fval.Fval
		}
		if sval := expr.AConst.GetSval(); sval != nil {
			return fmt.Sprintf("'%s'", sval.Sval)
		}
		if bsval := expr.AConst.GetBsval(); bsval != nil {
			return bsval.Bsval
		}

	case *pg_query.Node_FuncCall:
		// Handle function calls like NOW(), CURRENT_TIMESTAMP, datetime('now'), etc.
		if len(expr.FuncCall.Funcname) > 0 {
			if nameNode, ok := expr.FuncCall.Funcname[0].Node.(*pg_query.Node_String_); ok {
				funcName := nameNode.String_.Sval

				// Format arguments
				var args []string
				for _, argNode := range expr.FuncCall.Args {
					argStr := formatExpr(argNode)
					args = append(args, argStr)
				}

				if len(args) > 0 {
					return fmt.Sprintf("%s(%s)", funcName, strings.Join(args, ", "))
				}
				return funcName + "()"
			}
		}

	case *pg_query.Node_TypeCast:
		// Handle type casts
		if expr.TypeCast.Arg != nil {
			return formatExpr(expr.TypeCast.Arg)
		}

	case *pg_query.Node_SqlvalueFunction:
		// Handle SQL value functions like CURRENT_TIMESTAMP, CURRENT_USER, etc.
		// Based on PostgreSQL's SVFOp enum (1-indexed)
		// See: https://github.com/postgres/postgres/blob/master/src/include/nodes/primnodes.h
		switch expr.SqlvalueFunction.Op {
		case 1: // SVFOP_CURRENT_DATE
			return "CURRENT_DATE"
		case 2: // SVFOP_CURRENT_TIME
			return "CURRENT_TIME"
		case 3: // SVFOP_CURRENT_TIME_N (CURRENT_TIME with precision)
			return "CURRENT_TIME"
		case 4: // SVFOP_CURRENT_TIMESTAMP
			return "CURRENT_TIMESTAMP"
		case 5: // SVFOP_CURRENT_TIMESTAMP_N (CURRENT_TIMESTAMP with precision)
			return "CURRENT_TIMESTAMP"
		case 6: // SVFOP_LOCALTIME
			return "LOCALTIME"
		case 7: // SVFOP_LOCALTIME_N (LOCALTIME with precision)
			return "LOCALTIME"
		case 8: // SVFOP_LOCALTIMESTAMP
			return "LOCALTIMESTAMP"
		case 9: // SVFOP_LOCALTIMESTAMP_N (LOCALTIMESTAMP with precision)
			return "LOCALTIMESTAMP"
		case 10: // SVFOP_CURRENT_ROLE
			return "CURRENT_ROLE"
		case 11: // SVFOP_CURRENT_USER
			return "CURRENT_USER"
		case 12: // SVFOP_USER
			return "USER"
		case 13: // SVFOP_SESSION_USER
			return "SESSION_USER"
		case 14: // SVFOP_CURRENT_CATALOG
			return "CURRENT_CATALOG"
		case 15: // SVFOP_CURRENT_SCHEMA
			return "CURRENT_SCHEMA"
		}
	}

	// For anything else, return a placeholder
	return "DEFAULT"
}

// ExtractTableNameFromAlter extracts table name from ALTER TABLE statement
func ExtractTableNameFromAlter(sql string) (string, error) {
	// Pattern: ALTER TABLE <name> ...
	re := regexp.MustCompile(`ALTER\s+TABLE\s+(\w+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract table name from: %s", sql)
	}
	return matches[1], nil
}
