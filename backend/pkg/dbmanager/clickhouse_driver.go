package dbmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"strings"
	"time"

	clickhousedriver "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ClickHouseDriver implements the DatabaseDriver interface for ClickHouse
type ClickHouseDriver struct{}

// NewClickHouseDriver creates a new ClickHouse driver
func NewClickHouseDriver() DatabaseDriver {
	return &ClickHouseDriver{}
}

// ClickHouse schema structures
type ClickHouseSchema struct {
	Tables       map[string]ClickHouseTable
	Views        map[string]ClickHouseView
	Dictionaries map[string]ClickHouseDictionary
}

type ClickHouseTable struct {
	Name         string
	Columns      map[string]ClickHouseColumn
	Engine       string
	PartitionKey string
	OrderBy      string
	PrimaryKey   []string
	RowCount     int64
}

type ClickHouseColumn struct {
	Name         string
	Type         string
	IsNullable   bool
	DefaultValue string
	Comment      string
}

type ClickHouseView struct {
	Name       string
	Definition string
}

type ClickHouseDictionary struct {
	Name       string
	Definition string
}

// Convert ClickHouseColumn to generic ColumnInfo
func (cc ClickHouseColumn) toColumnInfo() ColumnInfo {
	return ColumnInfo{
		Name:         cc.Name,
		Type:         cc.Type,
		IsNullable:   cc.IsNullable,
		DefaultValue: cc.DefaultValue,
		Comment:      cc.Comment,
	}
}

// Connect establishes a connection to a ClickHouse database
func (d *ClickHouseDriver) Connect(config ConnectionConfig) (*Connection, error) {
	// Validate connection parameters
	if config.Host == "" || config.Port == "" || config.Database == "" {
		return nil, fmt.Errorf("invalid connection parameters: host, port, and database are required")
	}

	// Build DSN (Data Source Name)
	// Use the native format for ClickHouse DSN
	var dsn string
	if config.Username != nil && *config.Username != "" {
		if config.Password != nil && *config.Password != "" {
			dsn = fmt.Sprintf("clickhouse://%s:%s@%s:%s/%s?dial_timeout=10s&max_execution_time=60",
				*config.Username, *config.Password, config.Host, config.Port, config.Database)
		} else {
			dsn = fmt.Sprintf("clickhouse://%s@%s:%s/%s?dial_timeout=10s&max_execution_time=60",
				*config.Username, config.Host, config.Port, config.Database)
		}
	} else {
		dsn = fmt.Sprintf("clickhouse://%s:%s/%s?dial_timeout=10s&max_execution_time=60",
			config.Host, config.Port, config.Database)
	}

	log.Printf("ClickHouseDriver -> Connect -> Connecting with DSN format: %s",
		strings.Replace(dsn, *config.Password, "******", -1))

	// Configure GORM logger
	gormLogger := logger.New(
		log.New(log.Writer(), "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// Open connection using the GORM ClickHouse driver
	db, err := gorm.Open(clickhousedriver.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		log.Printf("ClickHouseDriver -> Connect -> Connection failed: %v", err)
		return nil, fmt.Errorf("failed to connect to ClickHouse: %v", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %v", err)
	}

	// Set connection pool parameters
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test the connection with a simple query
	var result int
	if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
		log.Printf("ClickHouseDriver -> Connect -> Connection test failed: %v", err)
		return nil, fmt.Errorf("connection test failed: %v", err)
	}
	log.Printf("ClickHouseDriver -> Connect -> Connection established successfully")

	// Create connection object
	conn := &Connection{
		DB:          db,
		LastUsed:    time.Now(),
		Status:      StatusConnected,
		Config:      config,
		Subscribers: make(map[string]bool),
	}

	return conn, nil
}

// Disconnect closes a ClickHouse database connection
func (d *ClickHouseDriver) Disconnect(conn *Connection) error {
	if conn == nil || conn.DB == nil {
		return fmt.Errorf("no active connection to disconnect")
	}

	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	return sqlDB.Close()
}

// Ping checks if the ClickHouse connection is alive
func (d *ClickHouseDriver) Ping(conn *Connection) error {
	if conn == nil || conn.DB == nil {
		return fmt.Errorf("no active connection to ping")
	}

	// Get the underlying SQL DB
	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("ClickHouseDriver -> Ping -> Failed to get database connection: %v", err)
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	// First try standard ping
	if err := sqlDB.Ping(); err != nil {
		log.Printf("ClickHouseDriver -> Ping -> Standard ping failed: %v", err)
		return fmt.Errorf("ping failed: %v", err)
	}

	// Also execute a simple query to ensure the connection is fully functional
	var result int
	if err := conn.DB.Raw("SELECT 1").Scan(&result).Error; err != nil {
		log.Printf("ClickHouseDriver -> Ping -> Query test failed: %v", err)
		return fmt.Errorf("connection test query failed: %v", err)
	}

	log.Printf("ClickHouseDriver -> Ping -> Connection is healthy")
	return nil
}

// IsAlive checks if the ClickHouse connection is still valid
func (d *ClickHouseDriver) IsAlive(conn *Connection) bool {
	if conn == nil || conn.DB == nil {
		log.Printf("ClickHouseDriver -> IsAlive -> No connection or DB object")
		return false
	}

	// Get the underlying SQL DB
	sqlDB, err := conn.DB.DB()
	if err != nil {
		log.Printf("ClickHouseDriver -> IsAlive -> Failed to get database connection: %v", err)
		return false
	}

	// First try standard ping
	if err := sqlDB.Ping(); err != nil {
		log.Printf("ClickHouseDriver -> IsAlive -> Standard ping failed: %v", err)
		return false
	}

	// Also execute a simple query to ensure the connection is fully functional
	var result int
	if err := conn.DB.Raw("SELECT 1").Scan(&result).Error; err != nil {
		log.Printf("ClickHouseDriver -> IsAlive -> Query test failed: %v", err)
		return false
	}

	log.Printf("ClickHouseDriver -> IsAlive -> Connection is healthy")
	return true
}

// ExecuteQuery executes a SQL query on the ClickHouse database
func (d *ClickHouseDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string) *QueryExecutionResult {
	if conn == nil || conn.DB == nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "No active connection",
				Code:    "CONNECTION_ERROR",
			},
		}
	}

	startTime := time.Now()
	result := &QueryExecutionResult{}

	// Split the query into individual statements
	statements := splitClickHouseStatements(query)

	// Execute each statement
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			result.Error = &dtos.QueryError{
				Message: "Query execution cancelled",
				Code:    "EXECUTION_CANCELLED",
			}
			return result
		}

		// Execute the statement based on query type
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "SELECT") ||
			strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "SHOW") ||
			strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "DESCRIBE") {
			// For SELECT, SHOW, DESCRIBE queries, return the results
			var rows []map[string]interface{}
			if err := conn.DB.WithContext(ctx).Raw(stmt).Scan(&rows).Error; err != nil {
				result.Error = &dtos.QueryError{
					Message: err.Error(),
					Code:    "EXECUTION_ERROR",
				}
				return result
			}

			// Process the rows to ensure proper type handling
			processedRows := make([]map[string]interface{}, len(rows))
			for i, row := range rows {
				processedRow := make(map[string]interface{})
				for key, val := range row {
					// Handle different types properly
					switch v := val.(type) {
					case []byte:
						// Convert []byte to string
						processedRow[key] = string(v)
					case string:
						// Keep strings as is
						processedRow[key] = v
					case float64:
						// Keep numbers as is
						processedRow[key] = v
					case int64:
						// Keep integers as is
						processedRow[key] = v
					case bool:
						// Keep booleans as is
						processedRow[key] = v
					case nil:
						// Keep nulls as is
						processedRow[key] = nil
					default:
						// For other types, convert to string
						processedRow[key] = fmt.Sprintf("%v", v)
					}
				}
				processedRows[i] = processedRow
			}

			result.Result = map[string]interface{}{
				"results": processedRows,
			}
		} else {
			// For other queries (INSERT, CREATE, ALTER, etc.), execute and return affected rows
			execResult := conn.DB.WithContext(ctx).Exec(stmt)
			if execResult.Error != nil {
				result.Error = &dtos.QueryError{
					Message: execResult.Error.Error(),
					Code:    "EXECUTION_ERROR",
				}
				return result
			}

			rowsAffected := execResult.RowsAffected
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
	}

	// Calculate execution time
	executionTime := int(time.Since(startTime).Milliseconds())
	result.ExecutionTime = executionTime

	// Marshal the result to JSON
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

// splitClickHouseStatements splits a ClickHouse query string into individual statements
func splitClickHouseStatements(query string) []string {
	// Split by semicolons, but handle cases where semicolons are within quotes
	var statements []string
	var currentStmt strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, char := range query {
		switch char {
		case '\'', '"', '`':
			if inQuote && char == quoteChar {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quoteChar = char
			}
			currentStmt.WriteRune(char)
		case ';':
			if inQuote {
				currentStmt.WriteRune(char)
			} else {
				statements = append(statements, currentStmt.String())
				currentStmt.Reset()
			}
		default:
			currentStmt.WriteRune(char)
		}
	}

	// Add the last statement if there's anything left
	if currentStmt.Len() > 0 {
		statements = append(statements, currentStmt.String())
	}

	return statements
}

// BeginTx starts a new transaction
func (d *ClickHouseDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	if conn == nil || conn.DB == nil {
		log.Printf("ClickHouseDriver.BeginTx: Connection or DB is nil")
		return nil
	}

	// Start a new transaction
	tx := conn.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		log.Printf("Failed to begin transaction: %v", tx.Error)
		return nil
	}

	return &ClickHouseTransaction{
		tx:   tx,
		conn: conn,
	}
}

// ClickHouseTransaction implements the Transaction interface for ClickHouse
type ClickHouseTransaction struct {
	tx   *gorm.DB
	conn *Connection
}

// ExecuteQuery executes a query within a transaction
func (t *ClickHouseTransaction) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string) *QueryExecutionResult {
	if t.tx == nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "No active transaction",
				Code:    "TRANSACTION_ERROR",
			},
		}
	}

	startTime := time.Now()
	result := &QueryExecutionResult{}

	// Split the query into individual statements
	statements := splitClickHouseStatements(query)

	// Execute each statement
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			result.Error = &dtos.QueryError{
				Message: "Query execution cancelled",
				Code:    "EXECUTION_CANCELLED",
			}
			return result
		}

		// Execute the statement based on query type
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "SELECT") ||
			strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "SHOW") ||
			strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "DESCRIBE") {
			// For SELECT, SHOW, DESCRIBE queries, return the results
			var rows []map[string]interface{}
			if err := t.tx.WithContext(ctx).Raw(stmt).Scan(&rows).Error; err != nil {
				result.Error = &dtos.QueryError{
					Message: err.Error(),
					Code:    "EXECUTION_ERROR",
				}
				return result
			}

			// Process the rows to ensure proper type handling
			processedRows := make([]map[string]interface{}, len(rows))
			for i, row := range rows {
				processedRow := make(map[string]interface{})
				for key, val := range row {
					// Handle different types properly
					switch v := val.(type) {
					case []byte:
						// Convert []byte to string
						processedRow[key] = string(v)
					case string:
						// Keep strings as is
						processedRow[key] = v
					case float64:
						// Keep numbers as is
						processedRow[key] = v
					case int64:
						// Keep integers as is
						processedRow[key] = v
					case bool:
						// Keep booleans as is
						processedRow[key] = v
					case nil:
						// Keep nulls as is
						processedRow[key] = nil
					default:
						// For other types, convert to string
						processedRow[key] = fmt.Sprintf("%v", v)
					}
				}
				processedRows[i] = processedRow
			}

			result.Result = map[string]interface{}{
				"results": processedRows,
			}
		} else {
			// For other queries (INSERT, CREATE, ALTER, etc.), execute and return affected rows
			execResult := t.tx.WithContext(ctx).Exec(stmt)
			if execResult.Error != nil {
				result.Error = &dtos.QueryError{
					Message: execResult.Error.Error(),
					Code:    "EXECUTION_ERROR",
				}
				return result
			}

			rowsAffected := execResult.RowsAffected
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
	}

	// Calculate execution time
	executionTime := int(time.Since(startTime).Milliseconds())
	result.ExecutionTime = executionTime

	// Marshal the result to JSON
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

// Commit commits the transaction
func (t *ClickHouseTransaction) Commit() error {
	if t.tx == nil {
		return fmt.Errorf("no active transaction to commit")
	}
	return t.tx.Commit().Error
}

// Rollback rolls back the transaction
func (t *ClickHouseTransaction) Rollback() error {
	if t.tx == nil {
		return fmt.Errorf("no active transaction to rollback")
	}
	return t.tx.Rollback().Error
}

// GetSchema retrieves the database schema
func (d *ClickHouseDriver) GetSchema(ctx context.Context, db DBExecutor, selectedTables []string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a new ClickHouse schema fetcher
	fetcher := NewClickHouseSchemaFetcher(db)

	// Get the schema
	return fetcher.GetSchema(ctx, db, selectedTables)
}

// GetTableChecksum calculates a checksum for a table
func (d *ClickHouseDriver) GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseDriver -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	// Create a new ClickHouse schema fetcher
	fetcher := NewClickHouseSchemaFetcher(db)

	// Get the table checksum
	return fetcher.GetTableChecksum(ctx, db, table)
}

// FetchExampleRecords fetches example records from a table
func (d *ClickHouseDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("ClickHouseDriver -> FetchExampleRecords -> Context cancelled: %v", err)
		return nil, err
	}

	// Create a new ClickHouse schema fetcher
	fetcher := NewClickHouseSchemaFetcher(db)

	// Get example records
	return fetcher.FetchExampleRecords(ctx, db, table, limit)
}
