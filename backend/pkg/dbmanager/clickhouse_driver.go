package dbmanager

import (
	"context"
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
	var dsn string
	if config.Username != nil && *config.Username != "" {
		if config.Password != nil && *config.Password != "" {
			dsn = fmt.Sprintf("tcp://%s:%s?database=%s&username=%s&password=%s&read_timeout=10&write_timeout=20",
				config.Host, config.Port, config.Database, *config.Username, *config.Password)
		} else {
			dsn = fmt.Sprintf("tcp://%s:%s?database=%s&username=%s&read_timeout=10&write_timeout=20",
				config.Host, config.Port, config.Database, *config.Username)
		}
	} else {
		dsn = fmt.Sprintf("tcp://%s:%s?database=%s&read_timeout=10&write_timeout=20",
			config.Host, config.Port, config.Database)
	}

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

	sqlDB, err := conn.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	return sqlDB.Ping()
}

// IsAlive checks if the ClickHouse connection is still valid
func (d *ClickHouseDriver) IsAlive(conn *Connection) bool {
	if conn == nil || conn.DB == nil {
		return false
	}

	sqlDB, err := conn.DB.DB()
	if err != nil {
		return false
	}

	return sqlDB.Ping() == nil
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
			result.Result = map[string]interface{}{
				"rows": rows,
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
			result.Result = map[string]interface{}{
				"rowsAffected": rowsAffected,
			}
		}
	}

	// Calculate execution time
	executionTime := int(time.Since(startTime).Milliseconds())
	result.ExecutionTime = executionTime

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
			result.Result = map[string]interface{}{
				"rows": rows,
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
			result.Result = map[string]interface{}{
				"rowsAffected": rowsAffected,
			}
		}
	}

	// Calculate execution time
	executionTime := int(time.Since(startTime).Milliseconds())
	result.ExecutionTime = executionTime

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
