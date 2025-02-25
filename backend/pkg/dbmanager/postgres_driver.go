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
	log.Printf("PostgreSQL Driver -> Connect -> Starting with config: %+v", config)

	// If username or password is nil, set it to empty string
	if config.Username == nil {
		config.Username = utils.ToStringPtr("")
		log.Printf("PostgreSQL Driver -> Connect -> Set nil username to empty string")
	}
	if config.Password == nil {
		config.Password = utils.ToStringPtr("")
		log.Printf("PostgreSQL Driver -> Connect -> Set nil password to empty string")
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, *config.Username, *config.Password, config.Database)

	log.Printf("PostgreSQL Driver -> Connect -> Attempting connection with DSN: %s", dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("PostgreSQL Driver -> Connect -> Connection failed: %v", err)
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}

	log.Printf("PostgreSQL Driver -> Connect -> GORM connection successful")

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("PostgreSQL Driver -> Connect -> Failed to get underlying *sql.DB: %v", err)
		return nil, err
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Printf("PostgreSQL Driver -> Connect -> Connection pool configured")

	// Test connection with ping
	if err := sqlDB.Ping(); err != nil {
		log.Printf("PostgreSQL Driver -> Connect -> Ping failed: %v", err)
		return nil, fmt.Errorf("ping failed: %v", err)
	}

	log.Printf("PostgreSQL Driver -> Connect -> Connection verified with ping")

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
	log.Printf("PostgreSQL Driver -> ExecuteQuery -> Query: %v", query)
	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("PostgreSQL Driver -> ExecuteQuery -> Failed to get SQL connection: %v", err)
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

	log.Printf("PostgreSQL Driver -> ExecuteQuery -> Statements: %v", statements)
	// Execute each statement
	for _, stmt := range statements {
		if stmt = strings.TrimSpace(stmt); stmt == "" {
			continue
		}

		lastResult, lastError = sqlDB.QueryContext(ctx, stmt)
		if lastError != nil {
			log.Printf("PostgreSQL Driver -> ExecuteQuery -> Query execution failed: %v", lastError)
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
	log.Printf("PostgreSQL Driver -> BeginTx -> Starting transaction")
	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("PostgreSQL Driver -> BeginTx -> Failed to get SQL connection: %v", err)
		return nil
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("PostgreSQL Driver -> BeginTx -> Failed to begin transaction: %v", err)
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
				// Check if table exists before dropping
				var exists bool
				checkStmt := `SELECT EXISTS (
					SELECT FROM information_schema.tables 
					WHERE table_schema = 'public' 
					AND table_name = $1
				)`
				tableName := extractTableName(stmt)
				err = tx.tx.QueryRow(checkStmt, tableName).Scan(&exists)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Code:    "TABLE_CHECK_FAILED",
							Message: err.Error(),
							Details: fmt.Sprintf("Failed to check table existence: %s", tableName),
						},
					}
				}
				if !exists {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Code:    "TABLE_NOT_FOUND",
							Message: fmt.Sprintf("Table '%s' does not exist", tableName),
							Details: "Cannot drop a table that doesn't exist",
						},
					}
				}
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
		result.Result = map[string]interface{}{
			"rowsAffected": rowsAffected,
			"message":      fmt.Sprintf("%d row(s) affected", rowsAffected),
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

// Helper function to extract table name from DROP TABLE statement
func extractTableName(stmt string) string {
	stmt = strings.ToUpper(stmt)
	stmt = strings.TrimSpace(strings.TrimPrefix(stmt, "DROP TABLE IF EXISTS"))
	stmt = strings.TrimSpace(strings.TrimPrefix(stmt, "DROP TABLE"))
	parts := strings.Fields(stmt)
	if len(parts) > 0 {
		return strings.ToLower(strings.Trim(parts[0], ";"))
	}
	return ""
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

// Update GetSchema method to include row counts
func (d *PostgresDriver) GetSchema(ctx context.Context, db DBExecutor) (*SchemaInfo, error) {
	sqlDB := db.GetDB()
	if sqlDB == nil {
		return nil, fmt.Errorf("failed to get SQL DB connection")
	}

	// Get table information
	tablesQuery := `
		SELECT 
			t.table_name,
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			col_description((t.table_schema || '.' || t.table_name)::regclass::oid, c.ordinal_position) as column_comment,
			s.n_live_tup as row_count
		FROM information_schema.tables t
		JOIN information_schema.columns c ON t.table_name = c.table_name
		LEFT JOIN pg_stat_user_tables s ON t.table_name = s.relname
		WHERE t.table_schema = 'public' AND t.table_type = 'BASE TABLE'
		ORDER BY t.table_name, c.ordinal_position;
	`

	rows, err := sqlDB.QueryContext(ctx, tablesQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %v", err)
	}
	defer rows.Close()

	tables := make(map[string]PostgresTable)
	for rows.Next() {
		var (
			tableName, columnName, dataType, isNullable string
			columnDefault, columnComment                sql.NullString
			rowCount                                    int64
		)

		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable, &columnDefault, &columnComment, &rowCount); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		// Initialize table if not exists
		if _, exists := tables[tableName]; !exists {
			tables[tableName] = PostgresTable{
				Name:        tableName,
				Columns:     make(map[string]PostgresColumn),
				PrimaryKey:  make([]string, 0),
				ForeignKeys: make(map[string]PostgresForeignKey),
				RowCount:    rowCount,
			}
		}

		// Add column information
		table := tables[tableName]
		table.Columns[columnName] = PostgresColumn{
			Name:         columnName,
			Type:         dataType,
			IsNullable:   isNullable == "YES",
			DefaultValue: columnDefault.String,
			Comment:      columnComment.String,
		}
		tables[tableName] = table
	}

	// Get primary keys
	pkQuery := `
		SELECT 
			tc.table_name, 
			kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
		WHERE tc.table_schema = 'public' 
			AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY tc.table_name, kcu.ordinal_position;
	`

	pkRows, err := sqlDB.QueryContext(ctx, pkQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query primary keys: %v", err)
	}
	defer pkRows.Close()

	for pkRows.Next() {
		var tableName, columnName string
		if err := pkRows.Scan(&tableName, &columnName); err != nil {
			return nil, fmt.Errorf("failed to scan primary key row: %v", err)
		}

		if table, exists := tables[tableName]; exists {
			table.PrimaryKey = append(table.PrimaryKey, columnName)
			tables[tableName] = table
		}
	}

	// Get essential schema elements
	tables, err = d.getTables(ctx, sqlDB)
	if err != nil {
		return nil, err
	}

	indexes, err := d.getIndexes(ctx, sqlDB)
	if err != nil {
		return nil, err
	}

	views, err := d.getViews(ctx, sqlDB)
	if err != nil {
		return nil, err
	}

	// Get foreign keys
	foreignKeys, err := d.getForeignKeys(ctx, sqlDB)
	if err != nil {
		return nil, err
	}

	// Add foreign keys to tables
	for tableName, tableFKs := range foreignKeys {
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

// Update the column fetching query and struct
type columnInfo struct {
	TableName     string         `db:"table_name"`
	ColumnName    string         `db:"column_name"`
	DataType      string         `db:"data_type"`
	IsNullable    string         `db:"is_nullable"`
	ColumnDefault sql.NullString `db:"column_default"`
	Description   sql.NullString `db:"description"`
}

func (d *PostgresDriver) getTables(ctx context.Context, db *sql.DB) (map[string]PostgresTable, error) {
	query := `
		SELECT 
			table_name, 
			column_name, 
			data_type, 
			is_nullable, 
			column_default,
			col_description((table_schema || '.' || table_name)::regclass::oid, ordinal_position) as column_comment
		FROM information_schema.columns 
		WHERE table_schema = 'public'
		ORDER BY table_name, ordinal_position;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string]PostgresTable)
	for rows.Next() {
		var tableName, columnName, dataType, isNullable, columnDefault, columnComment sql.NullString
		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable, &columnDefault, &columnComment); err != nil {
			return nil, err
		}

		table, exists := tables[tableName.String]
		if !exists {
			table = PostgresTable{
				Name:       tableName.String,
				Columns:    make(map[string]PostgresColumn),
				PrimaryKey: make([]string, 0),
			}
		}

		table.Columns[columnName.String] = PostgresColumn{
			Name:         columnName.String,
			Type:         dataType.String,
			IsNullable:   isNullable.String == "YES",
			DefaultValue: columnDefault.String,
			Comment:      columnComment.String,
		}

		tables[tableName.String] = table
	}
	return tables, nil
}

func (d *PostgresDriver) getIndexes(ctx context.Context, db *sql.DB) (map[string][]PostgresIndex, error) {
	query := `
		SELECT 
			t.relname AS table_name,
			i.relname AS index_name,
			string_agg(a.attname, ',') AS column_names,
			ix.indisunique AS is_unique
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid
		WHERE a.attnum = ANY(ix.indkey)
		AND t.relkind = 'r'
		GROUP BY t.relname, i.relname, ix.indisunique;
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	indexes := make(map[string][]PostgresIndex)
	for rows.Next() {
		var (
			index          PostgresIndex
			columnNamesStr string
		)

		if err := rows.Scan(
			&index.TableName,
			&index.Name,
			&columnNamesStr,
			&index.IsUnique,
		); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %v", err)
		}

		index.Columns = strings.Split(columnNamesStr, ",")

		indexes[index.TableName] = append(indexes[index.TableName], index)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating index rows: %v", err)
	}

	return indexes, nil
}

func (d *PostgresDriver) getViews(ctx context.Context, db *sql.DB) (map[string]PostgresView, error) {
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
		var view PostgresView
		if err := rows.Scan(&view.Name, &view.Definition); err != nil {
			return nil, err
		}
		views[view.Name] = view
	}
	return views, nil
}

func (d *PostgresDriver) getForeignKeys(ctx context.Context, db *sql.DB) (map[string]map[string]PostgresForeignKey, error) {
	query := `
		SELECT 
			tc.table_name,
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS ref_table,
			ccu.column_name AS ref_column,
			rc.update_rule,
			rc.delete_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
		JOIN information_schema.referential_constraints rc ON rc.constraint_name = tc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY';
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fks := make(map[string]map[string]PostgresForeignKey)
	for rows.Next() {
		var tableName, constraintName, columnName, refTable, refColumn, updateRule, deleteRule string
		if err := rows.Scan(&tableName, &constraintName, &columnName, &refTable, &refColumn, &updateRule, &deleteRule); err != nil {
			return nil, err
		}

		if _, exists := fks[tableName]; !exists {
			fks[tableName] = make(map[string]PostgresForeignKey)
		}

		fks[tableName][constraintName] = PostgresForeignKey{
			Name:      constraintName,
			Column:    columnName,
			RefTable:  refTable,
			RefColumn: refColumn,
			OnUpdate:  updateRule,
			OnDelete:  deleteRule,
		}
	}

	return fks, nil
}

// Add GetTableChecksum method
func (d *PostgresDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	sqlDB := db.GetDB()
	if sqlDB == nil {
		return "", fmt.Errorf("failed to get SQL DB connection")
	}

	// Get table definition checksum
	query := `
		SELECT COALESCE(
			(SELECT md5(string_agg(column_definition, ',' ORDER BY ordinal_position))
			FROM (
				SELECT 
					ordinal_position,
					concat(
						column_name, ':', 
						data_type, ':', 
						is_nullable, ':', 
						coalesce(column_default, ''), ':',
						coalesce(col_description((table_schema || '.' || table_name)::regclass::oid, ordinal_position), '')
					) as column_definition
				FROM information_schema.columns 
				WHERE table_schema = 'public' AND table_name = $1
			) t),
			'no_columns'
		) as checksum;
	`

	var checksum string
	if err := sqlDB.QueryRowContext(ctx, query, table).Scan(&checksum); err != nil {
		return "", fmt.Errorf("failed to get table checksum: %v", err)
	}

	// Get indexes checksum
	indexQuery := `
		SELECT COALESCE(
			(SELECT md5(string_agg(index_definition, ',' ORDER BY index_name))
			FROM (
				SELECT 
					i.relname as index_name,
					concat(
						i.relname, ':', 
						array_to_string(array_agg(a.attname ORDER BY a.attnum), ','), ':',
						ix.indisunique
					) as index_definition
				FROM pg_class t
				JOIN pg_index ix ON t.oid = ix.indrelid
				JOIN pg_class i ON i.oid = ix.indexrelid
				JOIN pg_attribute a ON a.attrelid = t.oid
				WHERE a.attnum = ANY(ix.indkey)
				AND t.relname = $1
				GROUP BY i.relname, ix.indisunique
			) t),
			'no_indexes'
		) as checksum;
	`

	var indexChecksum string
	if err := sqlDB.QueryRowContext(ctx, indexQuery, table).Scan(&indexChecksum); err != nil {
		return "", fmt.Errorf("failed to get index checksum: %v", err)
	}

	// Get foreign keys checksum
	fkQuery := `
		SELECT COALESCE(
			(SELECT md5(string_agg(fk_definition, ',' ORDER BY constraint_name))
			FROM (
				SELECT 
					tc.constraint_name,
					concat(
						tc.constraint_name, ':',
						kcu.column_name, ':',
						ccu.table_name, ':',
						ccu.column_name, ':',
						rc.update_rule, ':',
						rc.delete_rule
					) as fk_definition
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
				JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
				JOIN information_schema.referential_constraints rc ON rc.constraint_name = tc.constraint_name
				WHERE tc.table_name = $1 AND tc.constraint_type = 'FOREIGN KEY'
			) t),
			'no_foreign_keys'
		) as checksum;
	`

	var fkChecksum string
	if err := sqlDB.QueryRowContext(ctx, fkQuery, table).Scan(&fkChecksum); err != nil {
		return "", fmt.Errorf("failed to get foreign key checksum: %v", err)
	}

	// Combine all checksums
	finalChecksum := fmt.Sprintf("%s:%s:%s", checksum, indexChecksum, fkChecksum)
	return utils.MD5Hash(finalChecksum), nil
}
