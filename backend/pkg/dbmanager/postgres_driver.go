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

	sqlDB, err := conn.DB.DB()
	if err != nil {
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "FAILED_TO_GET_SQL_CONNECTION",
				Message: "Failed to get SQL connection",
				Details: err.Error(),
			},
		}
	}

	rows, err := sqlDB.QueryContext(ctx, query)
	if err != nil {
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "QUERY_EXECUTION_FAILED",
				Message: "Query execution failed",
				Details: err.Error(),
			},
		}
	}
	defer rows.Close()

	result := d.processRows(rows, startTime)

	// If DDL, trigger schema check
	if queryType == QueryTypeDDL {
		// Notify manager to update schema
		if conn.OnSchemaChange != nil {
			go conn.OnSchemaChange(conn.ChatID)
		}
	}

	return result
}

func (d *PostgresDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	sqlDB, err := conn.DB.DB()
	if err != nil {
		return nil
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}

	return &PostgresTransaction{tx: tx}
}

// Add PostgreSQL transaction implementation
type PostgresTransaction struct {
	tx *sql.Tx
}

func (t *PostgresTransaction) ExecuteQuery(ctx context.Context, query string, queryType string) *QueryExecutionResult {
	startTime := time.Now()

	rows, err := t.tx.QueryContext(ctx, query)
	if err != nil {
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "QUERY_ERROR",
				Message: "Query execution failed",
				Details: err.Error(),
			},
		}
	}
	defer rows.Close()

	return (&PostgresDriver{}).processRows(rows, startTime)
}

func (t *PostgresTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *PostgresTransaction) Rollback() error {
	return t.tx.Rollback()
}

// Helper method to process rows
func (d *PostgresDriver) processRows(rows *sql.Rows, startTime time.Time) *QueryExecutionResult {
	log.Printf("PostgreSQL Driver -> processRows -> Starting to process rows")
	columns, err := rows.Columns()
	log.Printf("PostgreSQL Driver -> processRows -> Columns: %+v", columns)
	if err != nil {
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "METADATA_ERROR",
				Message: "Failed to get column metadata",
				Details: err.Error(),
			},
		}
	}

	result := make([]map[string]interface{}, 0)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return &QueryExecutionResult{
				ExecutionTime: int(time.Since(startTime).Milliseconds()),
				Error: &dtos.QueryError{
					Code:    "SCAN_ERROR",
					Message: "Failed to scan row data",
					Details: err.Error(),
				},
			}
		}

		entry := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			entry[col] = v
		}
		result = append(result, entry)
	}

	log.Printf("PostgreSQL Driver -> processRows -> Result: %+v", result)

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &QueryExecutionResult{
			ExecutionTime: int(time.Since(startTime).Milliseconds()),
			Error: &dtos.QueryError{
				Code:    "JSON_ERROR",
				Message: "Failed to marshal result to JSON",
				Details: err.Error(),
			},
		}
	}

	log.Printf("PostgreSQL Driver -> processRows -> Result JSON: %s", string(resultJSON))

	var resultMap map[string]interface{}
	if len(result) > 0 {
		resultMap = result[0]
	} else {
		resultMap = make(map[string]interface{})
	}

	log.Printf("PostgreSQL Driver -> processRows -> Result Map: %+v", resultMap)

	return &QueryExecutionResult{
		Result:        resultMap,
		ResultJSON:    string(resultJSON),
		ExecutionTime: int(time.Since(startTime).Milliseconds()),
	}
}

// Update GetSchema to include foreign keys
func (d *PostgresDriver) GetSchema(ctx context.Context, db DBExecutor) (*SchemaInfo, error) {
	sqlDB := db.GetDB()
	if sqlDB == nil {
		return nil, fmt.Errorf("failed to get SQL DB connection")
	}

	// Get columns
	columnsQuery := `
		SELECT 
			c.table_name,
			c.column_name,
			c.data_type,
			c.is_nullable,
			c.column_default,
			pd.description
		FROM information_schema.columns c
		LEFT JOIN pg_description pd ON 
			pd.objoid = (quote_ident(c.table_name)::regclass)::oid AND 
			pd.objsubid = c.ordinal_position
		WHERE c.table_schema = 'public'
		ORDER BY c.table_name, c.ordinal_position;
	`

	rows, err := sqlDB.QueryContext(ctx, columnsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch columns: %v", err)
	}
	defer rows.Close()

	// Build schema
	schema := &SchemaInfo{
		Tables: make(map[string]TableSchema),
	}

	// Process columns
	for rows.Next() {
		var col columnInfo
		if err := rows.Scan(
			&col.TableName,
			&col.ColumnName,
			&col.DataType,
			&col.IsNullable,
			&col.ColumnDefault,
			&col.Description,
		); err != nil {
			return nil, fmt.Errorf("failed to scan column: %v", err)
		}

		table, exists := schema.Tables[col.TableName]
		if !exists {
			table = TableSchema{
				Name:        col.TableName,
				Columns:     make(map[string]ColumnInfo),
				Indexes:     make(map[string]IndexInfo),
				ForeignKeys: make(map[string]ForeignKey),
				Constraints: make(map[string]ConstraintInfo),
			}
		}

		// Add column
		table.Columns[col.ColumnName] = ColumnInfo{
			Name:         col.ColumnName,
			Type:         col.DataType,
			IsNullable:   col.IsNullable == "YES",
			DefaultValue: col.ColumnDefault.String,
			Comment:      col.Description.String,
		}

		schema.Tables[col.TableName] = table
	}

	// Get essential schema elements
	tables, err := d.getTables(ctx, sqlDB)
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

// Update convertToSchemaInfo to use the conversion method
func (d *PostgresDriver) convertToSchemaInfo(tables map[string]PostgresTable, indexes map[string][]PostgresIndex, views map[string]PostgresView) *SchemaInfo {
	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Convert tables and their components
	for tableName, table := range tables {
		tableSchema := TableSchema{
			Name:        tableName,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
		}

		// Convert columns using the conversion method
		for colName, col := range table.Columns {
			tableSchema.Columns[colName] = col.toColumnInfo()
		}

		// Convert indexes
		if tableIndexes, ok := indexes[tableName]; ok {
			for _, idx := range tableIndexes {
				tableSchema.Indexes[idx.Name] = IndexInfo{
					Name:     idx.Name,
					Columns:  idx.Columns,
					IsUnique: idx.IsUnique,
				}
			}
		}

		// Convert foreign keys
		if table.ForeignKeys != nil {
			for fkName, fk := range table.ForeignKeys {
				tableSchema.ForeignKeys[fkName] = ForeignKey{
					Name:       fk.Name,
					ColumnName: fk.Column,
					RefTable:   fk.RefTable,
					RefColumn:  fk.RefColumn,
					OnDelete:   fk.OnDelete,
					OnUpdate:   fk.OnUpdate,
				}
			}
		}

		schema.Tables[tableName] = tableSchema
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

// Add GetTableChecksum method
func (d *PostgresDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	sqlDB := db.GetDB()
	if sqlDB == nil {
		return "", fmt.Errorf("failed to get SQL DB connection")
	}

	// Get table definition checksum
	query := `
		SELECT md5(string_agg(column_definition, ',' ORDER BY ordinal_position)) as checksum
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
		) t;
	`

	var checksum string
	if err := sqlDB.QueryRowContext(ctx, query, table).Scan(&checksum); err != nil {
		return "", fmt.Errorf("failed to get table checksum: %v", err)
	}

	// Get indexes checksum
	indexQuery := `
		SELECT md5(string_agg(index_definition, ',' ORDER BY index_name)) as checksum
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
		) t;
	`

	var indexChecksum string
	if err := sqlDB.QueryRowContext(ctx, indexQuery, table).Scan(&indexChecksum); err != nil {
		if err != sql.ErrNoRows {
			return "", fmt.Errorf("failed to get index checksum: %v", err)
		}
		indexChecksum = "no_indexes"
	}

	// Get foreign keys checksum
	fkQuery := `
		SELECT md5(string_agg(fk_definition, ',' ORDER BY constraint_name)) as checksum
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
		) t;
	`

	var fkChecksum string
	if err := sqlDB.QueryRowContext(ctx, fkQuery, table).Scan(&fkChecksum); err != nil {
		if err != sql.ErrNoRows {
			return "", fmt.Errorf("failed to get foreign key checksum: %v", err)
		}
		fkChecksum = "no_foreign_keys"
	}

	// Combine all checksums
	finalChecksum := fmt.Sprintf("%s:%s:%s", checksum, indexChecksum, fkChecksum)
	return utils.MD5Hash(finalChecksum), nil
}
