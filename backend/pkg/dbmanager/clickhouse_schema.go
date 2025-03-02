package dbmanager

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ClickHouseSchemaFetcher implements schema fetching for ClickHouse
type ClickHouseSchemaFetcher struct {
	db DBExecutor
}

// NewClickHouseSchemaFetcher creates a new ClickHouse schema fetcher
func NewClickHouseSchemaFetcher(db DBExecutor) SchemaFetcher {
	return &ClickHouseSchemaFetcher{db: db}
}

// GetSchema retrieves the schema for the selected tables
func (f *ClickHouseSchemaFetcher) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Fetch the full schema
	schema, err := f.FetchSchema(ctx)
	if err != nil {
		return nil, err
	}

	// Filter the schema based on selected tables
	return f.filterSchemaForSelectedTables(schema, selectedTables), nil
}

// FetchSchema retrieves the full database schema
func (f *ClickHouseSchemaFetcher) FetchSchema(ctx context.Context) (*SchemaInfo, error) {
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

		// Fetch table engine and other metadata
		tableInfo, err := f.fetchTableInfo(ctx, table)
		if err != nil {
			return nil, err
		}

		// Add primary key as a constraint if it exists
		if len(tableInfo.PrimaryKey) > 0 {
			tableSchema.Constraints["PRIMARY"] = ConstraintInfo{
				Name:    "PRIMARY",
				Type:    "PRIMARY KEY",
				Columns: tableInfo.PrimaryKey,
			}
		}

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

// TableInfo holds additional ClickHouse table metadata
type TableInfo struct {
	Engine       string
	PartitionKey string
	OrderBy      string
	PrimaryKey   []string
}

// fetchTables retrieves all tables in the database
func (f *ClickHouseSchemaFetcher) fetchTables(_ context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT name 
        FROM system.tables 
        WHERE database = currentDatabase() 
        AND engine NOT LIKE 'View%'
        ORDER BY name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// fetchColumns retrieves all columns for a specific table
func (f *ClickHouseSchemaFetcher) fetchColumns(_ context.Context, table string) (map[string]ColumnInfo, error) {
	columns := make(map[string]ColumnInfo)
	var columnList []struct {
		Name         string `db:"name"`
		Type         string `db:"type"`
		DefaultType  string `db:"default_kind"`
		DefaultValue string `db:"default_expression"`
		Comment      string `db:"comment"`
	}

	query := `
        SELECT 
            name,
            type,
            default_kind,
            default_expression,
            comment
        FROM system.columns
        WHERE database = currentDatabase()
        AND table = ?
        ORDER BY position;
    `
	err := f.db.Query(query, &columnList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch columns for table %s: %v", table, err)
	}

	for _, col := range columnList {
		// In ClickHouse, columns are nullable if the type contains "Nullable"
		isNullable := strings.Contains(col.Type, "Nullable")

		// Format default value
		defaultValue := ""
		if col.DefaultType != "" && col.DefaultValue != "" {
			defaultValue = fmt.Sprintf("%s %s", col.DefaultType, col.DefaultValue)
		}

		columns[col.Name] = ColumnInfo{
			Name:         col.Name,
			Type:         col.Type,
			IsNullable:   isNullable,
			DefaultValue: defaultValue,
			Comment:      col.Comment,
		}
	}
	return columns, nil
}

// fetchTableInfo retrieves additional metadata for a table
func (f *ClickHouseSchemaFetcher) fetchTableInfo(_ context.Context, table string) (*TableInfo, error) {
	info := &TableInfo{}

	// Get engine
	var engine string
	engineQuery := `
        SELECT engine
        FROM system.tables
        WHERE database = currentDatabase()
        AND name = ?;
    `
	err := f.db.Query(engineQuery, &engine, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch engine for table %s: %v", table, err)
	}
	info.Engine = engine

	// Get partition key, order by, and primary key
	var tableSettings []struct {
		Name  string `db:"name"`
		Value string `db:"value"`
	}

	settingsQuery := `
        SELECT name, value
        FROM system.table_settings
        WHERE database = currentDatabase()
        AND table = ?
        AND name IN ('partition_key', 'sorting_key', 'primary_key');
    `
	err = f.db.Query(settingsQuery, &tableSettings, table)
	if err != nil {
		// Some engines don't have these settings, so we'll just continue
		return info, nil
	}

	for _, setting := range tableSettings {
		switch setting.Name {
		case "partition_key":
			info.PartitionKey = setting.Value
		case "sorting_key":
			info.OrderBy = setting.Value
		case "primary_key":
			// Primary key is a comma-separated list of columns
			if setting.Value != "" {
				info.PrimaryKey = strings.Split(setting.Value, ",")
				// Trim whitespace from each column name
				for i, col := range info.PrimaryKey {
					info.PrimaryKey[i] = strings.TrimSpace(col)
				}
			}
		}
	}

	return info, nil
}

// fetchViews retrieves all views in the database
func (f *ClickHouseSchemaFetcher) fetchViews(_ context.Context) (map[string]ViewSchema, error) {
	views := make(map[string]ViewSchema)
	var viewList []struct {
		Name       string `db:"name"`
		Definition string `db:"create_table_query"`
	}

	query := `
        SELECT 
            name,
            create_table_query
        FROM system.tables
        WHERE database = currentDatabase()
        AND engine LIKE 'View%'
        ORDER BY name;
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

// getTableRowCount gets the number of rows in a table
func (f *ClickHouseSchemaFetcher) getTableRowCount(_ context.Context, table string) (int64, error) {
	var count int64

	// First try to get from system.tables which is faster but approximate
	approxQuery := `
        SELECT 
            total_rows
        FROM system.tables
        WHERE database = currentDatabase()
        AND name = ?;
    `
	err := f.db.Query(approxQuery, &count, table)
	if err != nil || count == 0 {
		// If error or zero (which might mean the count is not available), try counting
		countQuery := fmt.Sprintf("SELECT count(*) FROM `%s`", table)
		err = f.db.Query(countQuery, &count)
		if err != nil {
			return 0, fmt.Errorf("failed to get row count for table %s: %v", table, err)
		}
	}
	return count, nil
}

// GetTableChecksum calculates a checksum for a table's structure
func (f *ClickHouseSchemaFetcher) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	// Get table definition
	var tableDefinition string
	query := `
        SELECT create_table_query
        FROM system.tables
        WHERE database = currentDatabase()
        AND name = ?;
    `

	err := db.Query(query, &tableDefinition, table)
	if err != nil {
		return "", fmt.Errorf("failed to get table definition: %v", err)
	}

	// Calculate checksum
	return fmt.Sprintf("%x", md5.Sum([]byte(tableDefinition))), nil
}

// FetchExampleRecords retrieves sample records from a table
func (f *ClickHouseSchemaFetcher) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
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
func (f *ClickHouseSchemaFetcher) FetchTableList(ctx context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT name 
        FROM system.tables 
        WHERE database = currentDatabase() 
        AND engine NOT LIKE 'View%'
        ORDER BY name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// filterSchemaForSelectedTables filters the schema to only include elements related to the selected tables
func (f *ClickHouseSchemaFetcher) filterSchemaForSelectedTables(schema *SchemaInfo, selectedTables []string) *SchemaInfo {
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
