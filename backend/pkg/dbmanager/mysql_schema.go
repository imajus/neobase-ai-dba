package dbmanager

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MySQLSchemaFetcher implements schema fetching for MySQL
type MySQLSchemaFetcher struct {
	db DBExecutor
}

// NewMySQLSchemaFetcher creates a new MySQL schema fetcher
func NewMySQLSchemaFetcher(db DBExecutor) SchemaFetcher {
	return &MySQLSchemaFetcher{db: db}
}

// GetSchema retrieves the schema for the selected tables
func (f *MySQLSchemaFetcher) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Fetch the full schema
	schema, err := f.FetchSchema(ctx)
	if err != nil {
		return nil, err
	}

	// Filter the schema based on selected tables
	return f.filterSchemaForSelectedTables(schema, selectedTables), nil
}

// FetchSchema retrieves the full database schema
func (f *MySQLSchemaFetcher) FetchSchema(ctx context.Context) (*SchemaInfo, error) {
	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Fetch tables
	tables, err := f.fetchTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
		tableSchema := TableSchema{
			Name:        table,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
		}

		// Fetch columns
		columns, err := f.fetchColumns(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.Columns = columns

		// Fetch indexes
		indexes, err := f.fetchIndexes(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.Indexes = indexes

		// Fetch foreign keys
		fkeys, err := f.fetchForeignKeys(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.ForeignKeys = fkeys

		// Fetch constraints
		constraints, err := f.fetchConstraints(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.Constraints = constraints

		// Get row count
		rowCount, err := f.getTableRowCount(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.RowCount = rowCount

		// Calculate table schema checksum
		tableData, _ := json.Marshal(tableSchema)
		tableSchema.Checksum = fmt.Sprintf("%x", md5.Sum(tableData))

		schema.Tables[table] = tableSchema
	}

	// Fetch views
	views, err := f.fetchViews(ctx)
	if err != nil {
		return nil, err
	}
	schema.Views = views

	// Calculate overall schema checksum
	schemaData, _ := json.Marshal(schema.Tables)
	schema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	return schema, nil
}

// fetchTables retrieves all tables in the database
func (f *MySQLSchemaFetcher) fetchTables(_ context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = DATABASE() 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// fetchColumns retrieves all columns for a specific table
func (f *MySQLSchemaFetcher) fetchColumns(_ context.Context, table string) (map[string]ColumnInfo, error) {
	columns := make(map[string]ColumnInfo)
	var columnList []struct {
		Name         string `db:"column_name"`
		Type         string `db:"data_type"`
		IsNullable   string `db:"is_nullable"`
		DefaultValue string `db:"column_default"`
		Comment      string `db:"column_comment"`
	}

	query := `
        SELECT 
            column_name,
            data_type,
            is_nullable,
            column_default,
            column_comment
        FROM information_schema.columns
        WHERE table_schema = DATABASE()
        AND table_name = ?
        ORDER BY ordinal_position;
    `
	err := f.db.Query(query, &columnList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch columns for table %s: %v", table, err)
	}

	for _, col := range columnList {
		columns[col.Name] = ColumnInfo{
			Name:         col.Name,
			Type:         col.Type,
			IsNullable:   col.IsNullable == "YES",
			DefaultValue: col.DefaultValue,
			Comment:      col.Comment,
		}
	}
	return columns, nil
}

// fetchIndexes retrieves all indexes for a specific table
func (f *MySQLSchemaFetcher) fetchIndexes(_ context.Context, table string) (map[string]IndexInfo, error) {
	indexes := make(map[string]IndexInfo)
	var indexList []struct {
		Name     string `db:"index_name"`
		Column   string `db:"column_name"`
		IsUnique bool   `db:"non_unique"`
	}

	query := `
        SELECT 
            index_name,
            column_name,
            non_unique = 0 as non_unique
        FROM information_schema.statistics
        WHERE table_schema = DATABASE()
        AND table_name = ?
        ORDER BY index_name, seq_in_index;
    `
	err := f.db.Query(query, &indexList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch indexes for table %s: %v", table, err)
	}

	// Group columns by index name
	indexColumns := make(map[string][]string)
	indexUnique := make(map[string]bool)
	for _, idx := range indexList {
		indexColumns[idx.Name] = append(indexColumns[idx.Name], idx.Column)
		indexUnique[idx.Name] = idx.IsUnique
	}

	// Create index info objects
	for name, columns := range indexColumns {
		indexes[name] = IndexInfo{
			Name:     name,
			Columns:  columns,
			IsUnique: indexUnique[name],
		}
	}
	return indexes, nil
}

// fetchForeignKeys retrieves all foreign keys for a specific table
func (f *MySQLSchemaFetcher) fetchForeignKeys(_ context.Context, table string) (map[string]ForeignKey, error) {
	fkeys := make(map[string]ForeignKey)
	var fkList []struct {
		Name       string `db:"constraint_name"`
		ColumnName string `db:"column_name"`
		RefTable   string `db:"referenced_table_name"`
		RefColumn  string `db:"referenced_column_name"`
		OnDelete   string `db:"delete_rule"`
		OnUpdate   string `db:"update_rule"`
	}

	query := `
        SELECT
            rc.constraint_name,
            kcu.column_name,
            kcu.referenced_table_name,
            kcu.referenced_column_name,
            rc.delete_rule,
            rc.update_rule
        FROM information_schema.referential_constraints rc
        JOIN information_schema.key_column_usage kcu
            ON kcu.constraint_name = rc.constraint_name
            AND kcu.constraint_schema = rc.constraint_schema
        WHERE rc.constraint_schema = DATABASE()
        AND kcu.table_name = ?;
    `
	err := f.db.Query(query, &fkList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch foreign keys for table %s: %v", table, err)
	}

	for _, fk := range fkList {
		fkeys[fk.Name] = ForeignKey{
			Name:       fk.Name,
			ColumnName: fk.ColumnName,
			RefTable:   fk.RefTable,
			RefColumn:  fk.RefColumn,
			OnDelete:   fk.OnDelete,
			OnUpdate:   fk.OnUpdate,
		}
	}
	return fkeys, nil
}

// fetchViews retrieves all views in the database
func (f *MySQLSchemaFetcher) fetchViews(_ context.Context) (map[string]ViewSchema, error) {
	views := make(map[string]ViewSchema)
	var viewList []struct {
		Name       string `db:"table_name"`
		Definition string `db:"view_definition"`
	}

	query := `
        SELECT 
            table_name,
            view_definition
        FROM information_schema.views
        WHERE table_schema = DATABASE()
        ORDER BY table_name;
    `
	err := f.db.Query(query, &viewList)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch views: %v", err)
	}

	for _, view := range viewList {
		views[view.Name] = ViewSchema{
			Name:       view.Name,
			Definition: view.Definition,
		}
	}
	return views, nil
}

// fetchConstraints retrieves all constraints for a specific table
func (f *MySQLSchemaFetcher) fetchConstraints(_ context.Context, table string) (map[string]ConstraintInfo, error) {
	constraints := make(map[string]ConstraintInfo)

	// Get primary key constraints
	var pkColumns []string
	pkQuery := `
        SELECT 
            column_name
        FROM information_schema.key_column_usage
        WHERE table_schema = DATABASE()
        AND table_name = ?
        AND constraint_name = 'PRIMARY'
        ORDER BY ordinal_position;
    `
	err := f.db.Query(pkQuery, &pkColumns, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch primary key for table %s: %v", table, err)
	}

	if len(pkColumns) > 0 {
		constraints["PRIMARY"] = ConstraintInfo{
			Name:    "PRIMARY",
			Type:    "PRIMARY KEY",
			Columns: pkColumns,
		}
	}

	// Get unique constraints (excluding primary key)
	var uniqueList []struct {
		Name   string `db:"constraint_name"`
		Column string `db:"column_name"`
	}
	uniqueQuery := `
        SELECT 
            tc.constraint_name,
            kcu.column_name
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
            ON kcu.constraint_name = tc.constraint_name
            AND kcu.constraint_schema = tc.constraint_schema
            AND kcu.table_name = tc.table_name
        WHERE tc.constraint_schema = DATABASE()
        AND tc.table_name = ?
        AND tc.constraint_type = 'UNIQUE'
        ORDER BY tc.constraint_name, kcu.ordinal_position;
    `
	err = f.db.Query(uniqueQuery, &uniqueList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unique constraints for table %s: %v", table, err)
	}

	// Group columns by constraint name
	uniqueColumns := make(map[string][]string)
	for _, unique := range uniqueList {
		uniqueColumns[unique.Name] = append(uniqueColumns[unique.Name], unique.Column)
	}

	// Create constraint info objects for unique constraints
	for name, columns := range uniqueColumns {
		constraints[name] = ConstraintInfo{
			Name:    name,
			Type:    "UNIQUE",
			Columns: columns,
		}
	}

	// MySQL doesn't have CHECK constraints in older versions, but we can add support for them here
	// for MySQL 8.0+ if needed

	return constraints, nil
}

// getTableRowCount gets the number of rows in a table
func (f *MySQLSchemaFetcher) getTableRowCount(_ context.Context, table string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", table)
	err := f.db.Query(query, &count)
	if err != nil {
		// If error (e.g., table too large), use approximate count from information_schema
		approxQuery := `
            SELECT 
                table_rows
            FROM information_schema.tables
            WHERE table_schema = DATABASE()
            AND table_name = ?;
        `
		err = f.db.Query(approxQuery, &count, table)
		if err != nil {
			return 0, fmt.Errorf("failed to get row count for table %s: %v", table, err)
		}
	}
	return count, nil
}

// GetTableChecksum calculates a checksum for a table's structure
func (f *MySQLSchemaFetcher) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	// Get table definition
	var tableDefinition string
	query := `
        SELECT 
            CONCAT(
                'CREATE TABLE ', table_name, ' (\n',
                GROUP_CONCAT(
                    CONCAT(
                        '  ', column_name, ' ', column_type, 
                        IF(is_nullable = 'NO', ' NOT NULL', ''),
                        IF(column_default IS NOT NULL, CONCAT(' DEFAULT ', column_default), ''),
                        IF(extra != '', CONCAT(' ', extra), '')
                    )
                    ORDER BY ordinal_position
                    SEPARATOR ',\n'
                ),
                '\n);'
            ) as definition
        FROM information_schema.columns
        WHERE table_schema = DATABASE()
        AND table_name = ?
        GROUP BY table_name;
    `

	err := db.Query(query, &tableDefinition, table)
	if err != nil {
		return "", fmt.Errorf("failed to get table definition: %v", err)
	}

	// Get indexes
	var indexes []string
	indexQuery := `
        SELECT 
            CONCAT(
                IF(non_unique = 0, 'CREATE UNIQUE INDEX ', 'CREATE INDEX '),
                index_name, ' ON ', table_name, ' (',
                GROUP_CONCAT(
                    column_name
                    ORDER BY seq_in_index
                    SEPARATOR ', '
                ),
                ');'
            ) as index_definition
        FROM information_schema.statistics
        WHERE table_schema = DATABASE()
        AND table_name = ?
        AND index_name != 'PRIMARY'
        GROUP BY index_name;
    `

	err = db.Query(indexQuery, &indexes, table)
	if err != nil {
		return "", fmt.Errorf("failed to get indexes: %v", err)
	}

	// Get foreign keys
	var foreignKeys []string
	fkQuery := `
        SELECT 
            CONCAT(
                'ALTER TABLE ', table_name, ' ADD CONSTRAINT ', constraint_name,
                ' FOREIGN KEY (', column_name, ') REFERENCES ',
                referenced_table_name, ' (', referenced_column_name, ');'
            ) as fk_definition
        FROM information_schema.key_column_usage
        WHERE table_schema = DATABASE()
        AND table_name = ?
        AND referenced_table_name IS NOT NULL;
    `

	err = db.Query(fkQuery, &foreignKeys, table)
	if err != nil {
		return "", fmt.Errorf("failed to get foreign keys: %v", err)
	}

	// Combine all definitions
	fullDefinition := tableDefinition
	for _, idx := range indexes {
		fullDefinition += "\n" + idx
	}
	for _, fk := range foreignKeys {
		fullDefinition += "\n" + fk
	}

	// Calculate checksum
	return fmt.Sprintf("%x", md5.Sum([]byte(fullDefinition))), nil
}

// FetchExampleRecords retrieves sample records from a table
func (f *MySQLSchemaFetcher) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
	}

	// Build a simple query to fetch example records
	query := fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", table, limit)

	var records []map[string]interface{}
	err := db.QueryRows(query, &records)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
	}

	// If no records found, return empty slice
	if len(records) == 0 {
		return []map[string]interface{}{}, nil
	}

	return records, nil
}

// FetchTableList retrieves a list of all tables in the database
func (f *MySQLSchemaFetcher) FetchTableList(ctx context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = DATABASE() 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// filterSchemaForSelectedTables filters the schema to only include elements related to the selected tables
func (f *MySQLSchemaFetcher) filterSchemaForSelectedTables(schema *SchemaInfo, selectedTables []string) *SchemaInfo {
	// If no tables are selected or "ALL" is selected, return the full schema
	if len(selectedTables) == 0 || (len(selectedTables) == 1 && selectedTables[0] == "ALL") {
		return schema
	}

	// Create a map for quick lookup of selected tables
	selectedTablesMap := make(map[string]bool)
	for _, table := range selectedTables {
		selectedTablesMap[table] = true
	}

	// Create a new filtered schema
	filteredSchema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: schema.UpdatedAt,
	}

	// Filter tables
	for tableName, tableSchema := range schema.Tables {
		if selectedTablesMap[tableName] {
			filteredSchema.Tables[tableName] = tableSchema
		}
	}

	// Collect all foreign key references to include related tables
	referencedTables := make(map[string]bool)
	for _, tableSchema := range filteredSchema.Tables {
		for _, fk := range tableSchema.ForeignKeys {
			referencedTables[fk.RefTable] = true
		}
	}

	// Add referenced tables that weren't in the original selection
	for refTable := range referencedTables {
		if !selectedTablesMap[refTable] {
			if tableSchema, ok := schema.Tables[refTable]; ok {
				filteredSchema.Tables[refTable] = tableSchema
			}
		}
	}

	// Filter views based on their definition referencing selected tables
	for viewName, viewSchema := range schema.Views {
		shouldInclude := false

		// Check if the view definition references any of the selected tables
		for tableName := range selectedTablesMap {
			if strings.Contains(strings.ToLower(viewSchema.Definition), strings.ToLower(tableName)) {
				shouldInclude = true
				break
			}
		}

		if shouldInclude {
			filteredSchema.Views[viewName] = viewSchema
		}
	}

	// Recalculate checksum for the filtered schema
	schemaData, _ := json.Marshal(filteredSchema.Tables)
	filteredSchema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	return filteredSchema
}
