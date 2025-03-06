package dbmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresDriver struct{}

func NewPostgresDriver() DatabaseDriver {
	return &PostgresDriver{}
}

// Add these constants
const (
	QueryTypeUnknown = "UNKNOWN"
	QueryTypeDDL     = "DDL"
	QueryTypeDML     = "DML"
	QueryTypeSelect  = "SELECT"
)

// Add these types for PostgreSQL schema tracking
type PostgresSchema struct {
	Tables      map[string]PostgresTable
	Indexes     map[string][]PostgresIndex
	Views       map[string]PostgresView
	Sequences   map[string]PostgresSequence     // For auto-increment/serial
	Constraints map[string][]PostgresConstraint // Table constraints (CHECK, UNIQUE, etc.)
	Enums       map[string]PostgresEnum         // Custom enum types
}

type PostgresTable struct {
	Name        string
	Columns     map[string]PostgresColumn
	Indexes     map[string]PostgresIndex
	PrimaryKey  []string
	ForeignKeys map[string]PostgresForeignKey
	RowCount    int64
}

type PostgresColumn struct {
	Name         string
	Type         string
	IsNullable   bool
	DefaultValue string
	Comment      string
}

type PostgresIndex struct {
	Name      string
	Columns   []string
	IsUnique  bool
	TableName string
}

type PostgresView struct {
	Name       string
	Definition string
}

type PostgresForeignKey struct {
	Name      string
	Column    string
	RefTable  string
	RefColumn string
	OnDelete  string
	OnUpdate  string
}

// Add new types for additional schema elements
type PostgresSequence struct {
	Name       string
	StartValue int64
	Increment  int64
	MinValue   int64
	MaxValue   int64
	CacheSize  int64
	IsCycled   bool
}

type PostgresConstraint struct {
	Name       string
	Type       string // CHECK, UNIQUE, etc.
	TableName  string
	Definition string
	Columns    []string
}

type PostgresEnum struct {
	Name   string
	Values []string
	Schema string
}

// Add a conversion method for PostgresColumn
func (pc PostgresColumn) toColumnInfo() ColumnInfo {
	return ColumnInfo{
		Name:         pc.Name,
		Type:         pc.Type,
		IsNullable:   pc.IsNullable,
		DefaultValue: pc.DefaultValue,
		Comment:      pc.Comment,
	}
}

func (d *PostgresDriver) Connect(config ConnectionConfig) (*Connection, error) {
	log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Starting with config: %+v", config)

	// If username or password is nil, set it to empty string
	if config.Username == nil {
		config.Username = utils.ToStringPtr("")
		log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Set nil username to empty string")
	}
	if config.Password == nil {
		config.Password = utils.ToStringPtr("")
		log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Set nil password to empty string")
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, *config.Username, *config.Password, config.Database)

	log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Attempting connection with DSN: %s", dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Connection failed: %v", err)
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}

	log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> GORM connection successful")

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Failed to get underlying *sql.DB: %v", err)
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Connection pool configured")

	// Test connection with ping
	if err := sqlDB.Ping(); err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Ping failed: %v", err)
		return nil, fmt.Errorf("ping failed: %v", err)
	}

	log.Printf("PostgreSQL/YugabyteDB Driver -> Connect -> Connection verified with ping")

	return &Connection{
		DB:       db,
		LastUsed: time.Now(),
		Status:   StatusConnected,
		Config:   config,
	}, nil
}

func (d *PostgresDriver) Disconnect(conn *Connection) error {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d *PostgresDriver) Ping(conn *Connection) error {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (d *PostgresDriver) IsAlive(conn *Connection) bool {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return false
	}
	return sqlDB.Ping() == nil
}

// Modify ExecuteQuery to check for schema changes
func (d *PostgresDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string) *QueryExecutionResult {
	startTime := time.Now()
	log.Printf("PostgreSQL/YugabyteDB Driver -> ExecuteQuery -> Query: %v", query)
	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> ExecuteQuery -> Failed to get SQL connection: %v", err)
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "FAILED_TO_GET_SQL_CONNECTION",
				Message: "Failed to get SQL connection",
				Details: err.Error(),
			},
		}
	}

	// Split multiple statements
	statements := splitStatements(query)
	var lastResult *sql.Rows
	var lastError error

	log.Printf("PostgreSQL/YugabyteDB Driver -> ExecuteQuery -> Statements: %v", statements)
	// Execute each statement
	for _, stmt := range statements {
		if stmt = strings.TrimSpace(stmt); stmt == "" {
			continue
		}

		lastResult, lastError = sqlDB.QueryContext(ctx, stmt)
		if lastError != nil {
			log.Printf("PostgreSQL/YugabyteDB Driver -> ExecuteQuery -> Query execution failed: %v", lastError)
			return &QueryExecutionResult{
				ExecutionTime: int(time.Since(startTime).Milliseconds()),
				Error: &dtos.QueryError{
					Code:    "QUERY_EXECUTION_FAILED",
					Message: "Query execution failed",
					Details: lastError.Error(),
				},
			}
		}
		if lastResult != nil {
			defer lastResult.Close()
		}
	}

	// Process results from the last statement if it returned rows
	var result *QueryExecutionResult
	if lastResult != nil {
		results, err := processRows(lastResult, startTime)
		if err != nil {
			return &QueryExecutionResult{
				ExecutionTime: int(time.Since(startTime).Milliseconds()),
				Error: &dtos.QueryError{
					Code:    "RESULT_PROCESSING_FAILED",
					Message: err.Error(),
					Details: "Failed to process query results",
				},
			}
		}
		result = &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Result: map[string]interface{}{
				"results": results,
			},
		}
	} else {
		result = &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Result: map[string]interface{}{
				"message": "Query executed successfully",
			},
		}
	}

	return result
}

// Helper function to split SQL statements
func splitStatements(query string) []string {
	// Basic statement splitting - can be enhanced for more complex cases
	statements := strings.Split(query, ";")

	// Clean up statements
	var result []string
	for _, stmt := range statements {
		if stmt = strings.TrimSpace(stmt); stmt != "" {
			result = append(result, stmt)
		}
	}
	return result
}

func (d *PostgresDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	log.Printf("PostgreSQL/YugabyteDB Driver -> BeginTx -> Starting transaction")

	if conn == nil || conn.DB == nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> BeginTx: Connection or DB is nil")
		return nil
	}

	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> BeginTx -> Failed to get SQL connection: %v", err)
		return nil
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("PostgreSQL/YugabyteDB Driver -> BeginTx -> Failed to begin transaction: %v", err)
		return nil
	}

	// Pass connection to transaction
	return &PostgresTransaction{
		tx:   tx,
		conn: conn,
	}
}

// Update PostgresTransaction to handle schema updates
type PostgresTransaction struct {
	tx   *sql.Tx
	conn *Connection // Add connection reference
}

func (tx *PostgresTransaction) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string) *QueryExecutionResult {
	startTime := time.Now()

	// Split into individual statements
	statements := splitStatements(query)
	log.Printf("PostgreSQL Transaction -> ExecuteQuery -> Statements: %v", statements)

	var lastResult sql.Result
	var rows *sql.Rows
	var err error

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// For SELECT queries
		if strings.HasPrefix(strings.ToUpper(stmt), "SELECT") {
			rows, err = tx.tx.QueryContext(ctx, stmt)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Code:    "QUERY_EXECUTION_FAILED",
						Message: err.Error(),
						Details: fmt.Sprintf("Failed to execute SELECT: %s", stmt),
					},
				}
			}
		} else {
			// For non-SELECT queries
			lastResult, err = tx.tx.ExecContext(ctx, stmt)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Code:    "QUERY_EXECUTION_FAILED",
						Message: err.Error(),
						Details: fmt.Sprintf("Failed to execute %s: %s", queryType, stmt),
					},
				}
			}

			// Check for specific PostgreSQL errors
			if strings.Contains(strings.ToUpper(stmt), "DROP TABLE") {
				// Extract table name
				tableName := extractTableName(stmt)
				log.Printf("PostgresDriver -> ExecuteQuery -> Checking existence of table: %s", tableName)

				// Check if table exists before dropping
				var exists bool
				checkStmt := `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`

				err = tx.tx.QueryRow(checkStmt, tableName).Scan(&exists)
				if err != nil {
					log.Printf("PostgresDriver -> ExecuteQuery -> Error checking table existence: %v", err)
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Code:    "TABLE_NOT_FOUND",
							Message: err.Error(),
							Details: "Cannot drop a table that doesn't exist",
						},
					}
				}

				log.Printf("PostgresDriver -> ExecuteQuery -> Table '%s' exists, proceeding with DROP", tableName)
			}
		}
	}

	// Process results
	result := &QueryExecutionResult{
		ExecutionTime: int(time.Since(startTime).Milliseconds()),
	}

	if rows != nil {
		defer rows.Close()
		results, err := processRows(rows, startTime)
		if err != nil {
			return &QueryExecutionResult{
				ExecutionTime: int(time.Since(startTime).Milliseconds()),
				Error: &dtos.QueryError{
					Code:    "RESULT_PROCESSING_FAILED",
					Message: err.Error(),
					Details: "Failed to process query results",
				},
			}
		}
		result.Result = map[string]interface{}{
			"results": results,
		}
	} else if lastResult != nil {
		rowsAffected, _ := lastResult.RowsAffected()
		if rowsAffected > 0 {
			result.Result = map[string]interface{}{
				"rowsAffected": rowsAffected,
				"message":      fmt.Sprintf("%d row(s) affected", rowsAffected),
			}
		} else {
			result.Result = map[string]interface{}{
				"message": "Query performed successfully",
			}
		}
	}

	resultJSON, err := json.Marshal(result.Result)
	if err != nil {
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "JSON_MARSHAL_FAILED",
				Message: err.Error(),
				Details: "Failed to marshal query results",
			},
		}
	}
	result.ResultJSON = string(resultJSON)

	return result
}

// Fix the extractTableName function to properly handle table names
func extractTableName(stmt string) string {
	// Preserve original case for the extraction
	originalStmt := strings.TrimSpace(stmt)
	upperStmt := strings.ToUpper(originalStmt)

	var tableName string

	// Handle different DROP TABLE variations
	if strings.HasPrefix(upperStmt, "DROP TABLE IF EXISTS") {
		tableName = strings.TrimSpace(originalStmt[len("DROP TABLE IF EXISTS"):])
	} else if strings.HasPrefix(upperStmt, "DROP TABLE") {
		tableName = strings.TrimSpace(originalStmt[len("DROP TABLE"):])
	} else {
		return ""
	}

	// Handle schema prefixes, quotes, and trailing characters
	tableName = strings.Split(tableName, " ")[0] // Get first word
	tableName = strings.Trim(tableName, "\"`;")  // Remove quotes and semicolons

	// Handle schema prefixes like "public."
	if strings.Contains(tableName, ".") {
		parts := strings.Split(tableName, ".")
		tableName = parts[len(parts)-1] // Get the last part after the dot
	}

	log.Printf("PostgresDriver -> extractTableName -> Extracted table name '%s' from statement: %s",
		tableName, originalStmt)

	return tableName
}

func (t *PostgresTransaction) Commit() error {
	log.Printf("PostgreSQL Transaction -> Commit -> Committing transaction")
	return t.tx.Commit()
}

func (t *PostgresTransaction) Rollback() error {
	log.Printf("PostgreSQL Transaction -> Rollback -> Rolling back transaction")
	return t.tx.Rollback()
}

// Update the processRows function signature to return results and error
func processRows(rows *sql.Rows, startTime time.Time) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}

	results := make([]map[string]interface{}, 0)
	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(columns))

	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(scanArgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val == nil {
				row[col] = nil
				continue
			}

			// Handle different types
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return results, nil
}

// Improve the GetSchema method to properly detect all tables
func (d *PostgresDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	sqlDB := db.GetDB()
	if sqlDB == nil {
		return nil, fmt.Errorf("failed to get SQL DB connection")
	}

	// Get all tables in the database, filtered by selectedTables if provided
	var tableQuery string
	var args []interface{}

	if len(selectedTables) > 0 && selectedTables[0] != "ALL" {
		// Build a query with a WHERE IN clause for selected tables
		placeholders := make([]string, len(selectedTables))
		args = make([]interface{}, len(selectedTables))

		for i, table := range selectedTables {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = table
		}

		tableQuery = fmt.Sprintf(`
			SELECT tablename 
			FROM pg_catalog.pg_tables 
			WHERE schemaname = 'public'
			AND tablename IN (%s);
		`, strings.Join(placeholders, ","))
	} else {
		// Get all tables
		tableQuery = `
			SELECT tablename 
			FROM pg_catalog.pg_tables 
			WHERE schemaname = 'public';
		`
	}

	var tableRows *sql.Rows
	var err error

	if len(args) > 0 {
		tableRows, err = sqlDB.QueryContext(ctx, tableQuery, args...)
	} else {
		tableRows, err = sqlDB.QueryContext(ctx, tableQuery)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %v", err)
	}
	defer tableRows.Close()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a list of all tables
	allTables := make([]string, 0)
	for tableRows.Next() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
			return nil, err
		}

		var tableName string
		if err := tableRows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %v", err)
		}
		allTables = append(allTables, tableName)
	}

	log.Printf("PostgresDriver -> GetSchema -> Found %d tables in database: %v", len(allTables), allTables)

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Continue with the rest of the schema fetching...
	tables, err := d.getTables(ctx, sqlDB, allTables)
	if err != nil {
		return nil, err
	}

	// Verify that all tables were properly fetched
	for _, tableName := range allTables {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
			return nil, err
		}

		if _, exists := tables[tableName]; !exists {
			log.Printf("PostgresDriver -> GetSchema -> Warning: Table %s exists but wasn't properly fetched", tableName)
		}
	}

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Continue with the rest of the schema fetching...
	indexes, err := d.getIndexes(ctx, sqlDB, allTables)
	if err != nil {
		return nil, err
	}

	views, err := d.getViews(ctx, sqlDB)
	if err != nil {
		return nil, err
	}

	// Get foreign keys
	foreignKeys, err := d.getForeignKeys(ctx, sqlDB, allTables)
	if err != nil {
		return nil, err
	}

	// Add foreign keys to tables
	for tableName, tableFKs := range foreignKeys {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> GetSchema -> Context cancelled: %v", err)
			return nil, err
		}

		if table, exists := tables[tableName]; exists {
			table.ForeignKeys = tableFKs
			tables[tableName] = table
		}
	}

	// Convert to generic SchemaInfo
	return d.convertToSchemaInfo(tables, indexes, views), nil
}

// Update the function signature to include indexes
func convertTablesToSchemaFormat(tables map[string]PostgresTable, indexes map[string][]PostgresIndex) map[string]TableSchema {
	result := make(map[string]TableSchema)
	for name, table := range tables {
		schema := TableSchema{
			Name:        name,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
			RowCount:    table.RowCount,
		}

		// Convert columns
		for colName, col := range table.Columns {
			schema.Columns[colName] = col.toColumnInfo()
		}

		// Convert indexes
		if tableIndexes, ok := indexes[name]; ok {
			for _, idx := range tableIndexes {
				schema.Indexes[idx.Name] = IndexInfo{
					Name:     idx.Name,
					Columns:  idx.Columns,
					IsUnique: idx.IsUnique,
				}
			}
		}

		// Convert foreign keys
		if table.ForeignKeys != nil {
			for fkName, fk := range table.ForeignKeys {
				schema.ForeignKeys[fkName] = ForeignKey{
					Name:       fk.Name,
					ColumnName: fk.Column,
					RefTable:   fk.RefTable,
					RefColumn:  fk.RefColumn,
					OnDelete:   fk.OnDelete,
					OnUpdate:   fk.OnUpdate,
				}
			}
		}

		result[name] = schema
	}
	return result
}

// Update the convertToSchemaInfo function to pass indexes
func (d *PostgresDriver) convertToSchemaInfo(tables map[string]PostgresTable, indexes map[string][]PostgresIndex, views map[string]PostgresView) *SchemaInfo {
	schema := &SchemaInfo{
		Tables:    convertTablesToSchemaFormat(tables, indexes),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Convert views
	for viewName, view := range views {
		schema.Views[viewName] = ViewSchema{
			Name:       viewName,
			Definition: view.Definition,
		}
	}

	return schema
}

// Improve the getTables method to properly fetch column details
func (d *PostgresDriver) getTables(ctx context.Context, db *sql.DB, tables []string) (map[string]PostgresTable, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a map to store all tables
	tablesMap := make(map[string]PostgresTable)
	allTableNames := make([]string, 0)

	// Initialize all tables first
	for _, tableName := range tables {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
			return nil, err
		}

		tablesMap[tableName] = PostgresTable{
			Name:        tableName,
			Columns:     make(map[string]PostgresColumn),
			Indexes:     make(map[string]PostgresIndex),
			ForeignKeys: make(map[string]PostgresForeignKey),
		}
		allTableNames = append(allTableNames, tableName)
	}

	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
		return nil, err
	}

	// For each table, get columns with SUPER DETAILED logging
	for tableName, table := range tablesMap {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
			return nil, err
		}

		log.Printf("PostgresDriver -> getTables -> Fetching columns for table: %s", tableName)

		// Get columns
		columnQuery := `
			SELECT 
				column_name, 
				data_type, 
				is_nullable,
				column_default,
				col_description((table_schema || '.' || table_name)::regclass::oid, ordinal_position) as column_comment
			FROM 
				information_schema.columns
			WHERE 
				table_schema = 'public' AND 
				table_name = $1
			ORDER BY 
				ordinal_position;
		`

		columnRows, err := db.QueryContext(ctx, columnQuery, tableName)
		if err != nil {
			log.Printf("PostgresDriver -> getTables -> Error fetching columns for table %s: %v", tableName, err)
			continue
		}

		columnCount := 0
		for columnRows.Next() {
			// Check for context cancellation
			if err := ctx.Err(); err != nil {
				log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
				return nil, err
			}

			var (
				columnName, dataType, isNullable string
				columnDefault, columnComment     sql.NullString
			)

			if err := columnRows.Scan(&columnName, &dataType, &isNullable, &columnDefault, &columnComment); err != nil {
				log.Printf("PostgresDriver -> getTables -> Error scanning column for table %s: %v", tableName, err)
				continue
			}

			// Convert is_nullable to bool
			isNullableBool := isNullable == "YES"

			// Get default value
			defaultValue := ""
			if columnDefault.Valid {
				defaultValue = columnDefault.String
			}

			// Get comment
			comment := ""
			if columnComment.Valid {
				comment = columnComment.String
			}

			log.Printf("PostgresDriver -> getTables -> Found column in table %s: name=%s, type=%s, nullable=%v, default=%s, comment=%s",
				tableName, columnName, dataType, isNullableBool, defaultValue, comment)

			table.Columns[columnName] = PostgresColumn{
				Name:         columnName,
				Type:         dataType,
				IsNullable:   isNullableBool,
				DefaultValue: defaultValue,
				Comment:      comment,
			}

			columnCount++
		}

		columnRows.Close()
		log.Printf("PostgresDriver -> getTables -> Fetched %d columns for table %s", columnCount, tableName)

		// Get indexes with SUPER DETAILED logging
		log.Printf("PostgresDriver -> getTables -> Fetching indexes for table: %s", tableName)
		indexQuery := `
			SELECT
				i.relname as index_name,
				array_to_string(array_agg(a.attname), ',') as column_names,
				ix.indisunique as is_unique,
				ix.indisprimary as is_primary
			FROM
				pg_class t,
				pg_class i,
				pg_index ix,
				pg_attribute a
			WHERE
				t.oid = ix.indrelid
				and i.oid = ix.indexrelid
				and a.attrelid = t.oid
				and a.attnum = ANY(ix.indkey)
				and t.relkind = 'r'
				and t.relname = $1
			GROUP BY
				i.relname,
				ix.indisunique,
				ix.indisprimary
			ORDER BY
				i.relname;
		`

		indexRows, err := db.QueryContext(ctx, indexQuery, tableName)
		if err != nil {
			log.Printf("PostgresDriver -> getTables -> Error fetching indexes for table %s: %v", tableName, err)
			continue
		}

		indexCount := 0
		for indexRows.Next() {
			// Check for context cancellation
			if err := ctx.Err(); err != nil {
				log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
				return nil, err
			}

			var (
				indexName, columnNames string
				isUnique, isPrimary    bool
			)

			if err := indexRows.Scan(&indexName, &columnNames, &isUnique, &isPrimary); err != nil {
				log.Printf("PostgresDriver -> getTables -> Error scanning index for table %s: %v", tableName, err)
				continue
			}

			log.Printf("PostgresDriver -> getTables -> Found index in table %s: name=%s, columns=%s, unique=%v, primary=%v",
				tableName, indexName, columnNames, isUnique, isPrimary)

			table.Indexes[indexName] = PostgresIndex{
				Name:     indexName,
				Columns:  strings.Split(columnNames, ","),
				IsUnique: isUnique,
			}

			indexCount++
		}

		indexRows.Close()
		log.Printf("PostgresDriver -> getTables -> Fetched %d indexes for table %s", indexCount, tableName)

		// Get foreign keys with SUPER DETAILED logging
		log.Printf("PostgresDriver -> getTables -> Fetching foreign keys for table: %s", tableName)
		fkQuery := `
			SELECT
				tc.constraint_name,
				kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name
			FROM
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				  ON tc.constraint_name = kcu.constraint_name
				JOIN information_schema.constraint_column_usage AS ccu
				  ON ccu.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1;
		`

		fkRows, err := db.QueryContext(ctx, fkQuery, tableName)
		if err != nil {
			log.Printf("PostgresDriver -> getTables -> Error fetching foreign keys for table %s: %v", tableName, err)
			continue
		}

		fkCount := 0
		for fkRows.Next() {
			// Check for context cancellation
			if err := ctx.Err(); err != nil {
				log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
				return nil, err
			}

			var (
				constraintName, columnName, foreignTableName, foreignColumnName string
			)

			if err := fkRows.Scan(&constraintName, &columnName, &foreignTableName, &foreignColumnName); err != nil {
				log.Printf("PostgresDriver -> getTables -> Error scanning foreign key for table %s: %v", tableName, err)
				continue
			}

			log.Printf("PostgresDriver -> getTables -> Found foreign key in table %s: name=%s, column=%s, references=%s.%s",
				tableName, constraintName, columnName, foreignTableName, foreignColumnName)

			table.ForeignKeys[constraintName] = PostgresForeignKey{
				Name:      constraintName,
				Column:    columnName,
				RefTable:  foreignTableName,
				RefColumn: foreignColumnName,
			}

			fkCount++
		}

		fkRows.Close()
		log.Printf("PostgresDriver -> getTables -> Fetched %d foreign keys for table %s", fkCount, tableName)

		// Update the table in the map
		tablesMap[tableName] = table
	}

	// Verify all tables were processed
	for _, tableName := range allTableNames {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getTables -> Context cancelled: %v", err)
			return nil, err
		}

		if _, exists := tablesMap[tableName]; !exists {
			log.Printf("PostgresDriver -> getTables -> Warning: Table %s exists but wasn't properly fetched", tableName)
		} else if len(tablesMap[tableName].Columns) == 0 {
			log.Printf("PostgresDriver -> getTables -> Warning: Table %s has no columns", tableName)
		}
	}

	log.Printf("PostgresDriver -> getTables -> Successfully fetched %d tables: %v", len(tablesMap), allTableNames)

	return tablesMap, nil
}

func (d *PostgresDriver) getIndexes(ctx context.Context, db *sql.DB, tables []string) (map[string][]PostgresIndex, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getIndexes -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a map to store indexes by table
	indexes := make(map[string][]PostgresIndex)

	// If no tables provided, return empty map
	if len(tables) == 0 {
		return indexes, nil
	}

	// Build query with table filter
	var query string
	var args []interface{}

	if len(tables) == 1 {
		// Simple case with one table
		query = `
			SELECT
				t.relname as table_name,
				i.relname as index_name,
				array_to_string(array_agg(a.attname), ',') as column_names,
				ix.indisunique as is_unique
			FROM
				pg_class t,
				pg_class i,
				pg_index ix,
				pg_attribute a
			WHERE
				t.oid = ix.indrelid
				and i.oid = ix.indexrelid
				and a.attrelid = t.oid
				and a.attnum = ANY(ix.indkey)
				and t.relkind = 'r'
				and t.relname = $1
			GROUP BY
				t.relname,
				i.relname,
				ix.indisunique
			ORDER BY
				t.relname,
				i.relname;
		`
		args = []interface{}{tables[0]}
	} else {
		// Multiple tables case
		placeholders := make([]string, len(tables))
		args = make([]interface{}, len(tables))

		for i, table := range tables {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = table
		}

		query = fmt.Sprintf(`
			SELECT
				t.relname as table_name,
				i.relname as index_name,
				array_to_string(array_agg(a.attname), ',') as column_names,
				ix.indisunique as is_unique
			FROM
				pg_class t,
				pg_class i,
				pg_index ix,
				pg_attribute a
			WHERE
				t.oid = ix.indrelid
				and i.oid = ix.indexrelid
				and a.attrelid = t.oid
				and a.attnum = ANY(ix.indkey)
				and t.relkind = 'r'
				and t.relname IN (%s)
			GROUP BY
				t.relname,
				i.relname,
				ix.indisunique
			ORDER BY
				t.relname,
				i.relname;
		`, strings.Join(placeholders, ","))
	}

	// Execute query
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getIndexes -> Context cancelled: %v", err)
			return nil, err
		}

		var (
			tableName, indexName, columnNames string
			isUnique                          bool
		)

		if err := rows.Scan(&tableName, &indexName, &columnNames, &isUnique); err != nil {
			return nil, fmt.Errorf("failed to scan index: %v", err)
		}

		// Create index
		index := PostgresIndex{
			Name:      indexName,
			Columns:   strings.Split(columnNames, ","),
			IsUnique:  isUnique,
			TableName: tableName,
		}

		// Add to map
		if _, exists := indexes[tableName]; !exists {
			indexes[tableName] = make([]PostgresIndex, 0)
		}
		indexes[tableName] = append(indexes[tableName], index)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating index rows: %v", err)
	}

	return indexes, nil
}

func (d *PostgresDriver) getViews(ctx context.Context, db *sql.DB) (map[string]PostgresView, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getViews -> Context cancelled: %v", err)
		return nil, err
	}

	query := `
		SELECT 
			viewname,
			definition
		FROM pg_views
		WHERE schemaname = 'public';
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	views := make(map[string]PostgresView)
	for rows.Next() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getViews -> Context cancelled: %v", err)
			return nil, err
		}

		var view PostgresView
		if err := rows.Scan(&view.Name, &view.Definition); err != nil {
			return nil, err
		}
		views[view.Name] = view
	}
	return views, nil
}

func (d *PostgresDriver) getForeignKeys(ctx context.Context, db *sql.DB, tables []string) (map[string]map[string]PostgresForeignKey, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> getForeignKeys -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a map to store foreign keys by table
	foreignKeys := make(map[string]map[string]PostgresForeignKey)

	// If no tables provided, return empty map
	if len(tables) == 0 {
		return foreignKeys, nil
	}

	// Build query with table filter
	var query string
	var args []interface{}

	if len(tables) == 1 {
		// Simple case with one table
		query = `
			SELECT
				tc.table_name,
				tc.constraint_name,
				kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name,
				rc.delete_rule,
				rc.update_rule
			FROM
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				  ON tc.constraint_name = kcu.constraint_name
				JOIN information_schema.constraint_column_usage AS ccu
				  ON ccu.constraint_name = tc.constraint_name
				JOIN information_schema.referential_constraints AS rc
				  ON rc.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1;
		`
		args = []interface{}{tables[0]}
	} else {
		// Multiple tables case
		placeholders := make([]string, len(tables))
		args = make([]interface{}, len(tables))

		for i, table := range tables {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = table
		}

		query = fmt.Sprintf(`
			SELECT
				tc.table_name,
				tc.constraint_name,
				kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name,
				rc.delete_rule,
				rc.update_rule
			FROM
				information_schema.table_constraints AS tc
				JOIN information_schema.key_column_usage AS kcu
				  ON tc.constraint_name = kcu.constraint_name
				JOIN information_schema.constraint_column_usage AS ccu
				  ON ccu.constraint_name = tc.constraint_name
				JOIN information_schema.referential_constraints AS rc
				  ON rc.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name IN (%s);
		`, strings.Join(placeholders, ","))
	}

	// Execute query
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %v", err)
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("PostgresDriver -> getForeignKeys -> Context cancelled: %v", err)
			return nil, err
		}

		var (
			tableName, constraintName, columnName, foreignTableName, foreignColumnName string
			deleteRule, updateRule                                                     string
		)

		if err := rows.Scan(&tableName, &constraintName, &columnName, &foreignTableName, &foreignColumnName, &deleteRule, &updateRule); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %v", err)
		}

		// Create foreign key
		fk := PostgresForeignKey{
			Name:      constraintName,
			Column:    columnName,
			RefTable:  foreignTableName,
			RefColumn: foreignColumnName,
			OnDelete:  deleteRule,
			OnUpdate:  updateRule,
		}

		// Add to map
		if _, exists := foreignKeys[tableName]; !exists {
			foreignKeys[tableName] = make(map[string]PostgresForeignKey)
		}
		foreignKeys[tableName][constraintName] = fk
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating foreign key rows: %v", err)
	}

	return foreignKeys, nil
}

// Fix the GetTableChecksum method to be more stable and consistent
func (d *PostgresDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresDriver -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	sqlDB := db.GetDB()
	if sqlDB == nil {
		return "", fmt.Errorf("failed to get SQL DB connection")
	}

	// Get table definition checksum - use a more stable approach that ignores non-structural changes
	query := `
		SELECT md5(string_agg(column_definition, ',' ORDER BY ordinal_position))
		FROM (
			SELECT 
				ordinal_position,
				concat(
					column_name, ':', 
					data_type, ':', 
					is_nullable, ':', 
					coalesce(column_default, '')
				) as column_definition
			FROM information_schema.columns 
			WHERE table_schema = 'public' AND table_name = $1
		) t;
	`

	var checksum string
	if err := sqlDB.QueryRowContext(ctx, query, table).Scan(&checksum); err != nil {
		return "", fmt.Errorf("failed to get table checksum: %v", err)
	}

	// Get indexes checksum - use a more stable approach that ignores index names
	indexQuery := `
		SELECT md5(string_agg(index_definition, ',' ORDER BY index_columns))
		FROM (
			SELECT 
				array_to_string(array_agg(a.attname ORDER BY a.attnum), ',') as index_columns,
				concat(
					array_to_string(array_agg(a.attname ORDER BY a.attnum), ','), ':',
					ix.indisunique
				) as index_definition
			FROM pg_class t
			JOIN pg_index ix ON t.oid = ix.indrelid
			JOIN pg_class i ON i.oid = ix.indexrelid
			JOIN pg_attribute a ON a.attrelid = t.oid
			WHERE a.attnum = ANY(ix.indkey)
			AND t.relname = $1
			GROUP BY ix.indexrelid, ix.indisunique
		) t;
	`

	var indexChecksum string
	if err := sqlDB.QueryRowContext(ctx, indexQuery, table).Scan(&indexChecksum); err != nil {
		return "", fmt.Errorf("failed to get index checksum: %v", err)
	}

	// Get foreign keys checksum - use a more stable approach that ignores constraint names
	fkQuery := `
		SELECT md5(string_agg(fk_definition, ',' ORDER BY source_column, target_table, target_column))
		FROM (
			SELECT 
				kcu.column_name as source_column,
				ccu.table_name as target_table,
				ccu.column_name as target_column,
				concat(
					kcu.column_name, ':',
					ccu.table_name, ':',
					ccu.column_name
				) as fk_definition
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
			WHERE tc.table_name = $1 AND tc.constraint_type = 'FOREIGN KEY'
		) t;
	`

	var fkChecksum string
	if err := sqlDB.QueryRowContext(ctx, fkQuery, table).Scan(&fkChecksum); err != nil {
		return "", fmt.Errorf("failed to get foreign key checksum: %v", err)
	}

	// Combine all checksums
	finalChecksum := fmt.Sprintf("%s:%s:%s", checksum, indexChecksum, fkChecksum)
	return utils.MD5Hash(finalChecksum), nil
}

// Add FetchExampleRecords method to PostgresDriver
func (d *PostgresDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
	}

	// Build a simple query to fetch example records
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d", table, limit)

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
