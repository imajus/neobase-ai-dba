package dbmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresDriver struct{}

func NewPostgresDriver() DatabaseDriver {
	return &PostgresDriver{}
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

func (d *PostgresDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string) *QueryExecutionResult {
	startTime := time.Now()

	sqlDB, err := conn.DB.DB()
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
	rows, err := sqlDB.QueryContext(ctx, query)
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

	return d.processRows(rows, startTime)
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

func (t *PostgresTransaction) ExecuteQuery(ctx context.Context, query string) *QueryExecutionResult {
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
	columns, err := rows.Columns()
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

	var resultMap map[string]interface{}
	if len(result) > 0 {
		resultMap = result[0]
	} else {
		resultMap = make(map[string]interface{})
	}

	return &QueryExecutionResult{
		Result:        resultMap,
		ResultJSON:    string(resultJSON),
		ExecutionTime: int(time.Since(startTime).Milliseconds()),
	}
}
