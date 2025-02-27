package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// DBExecutor interface defines common database operations
type DBExecutor interface {
	Raw(sql string, values ...interface{}) error
	Exec(sql string, values ...interface{}) error
	Query(sql string, dest interface{}, values ...interface{}) error
	Close() error
	GetDB() *sql.DB
	GetSchema(ctx context.Context) (*SchemaInfo, error)
	GetTableChecksum(ctx context.Context, table string) (string, error)
}

// BaseWrapper provides common functionality for all DB wrappers
type BaseWrapper struct {
	db      *gorm.DB
	manager *Manager
	chatID  string
}

func (w *BaseWrapper) updateUsage() error {
	if err := w.manager.UpdateLastUsed(w.chatID); err != nil {
		log.Printf("Failed to update last used time: %v", err)
		return err
	}
	return nil
}

// PostgresWrapper implements DBExecutor for PostgreSQL
type PostgresWrapper struct {
	BaseWrapper
}

func NewPostgresWrapper(db *gorm.DB, manager *Manager, chatID string) *PostgresWrapper {
	return &PostgresWrapper{
		BaseWrapper: BaseWrapper{
			db:      db,
			manager: manager,
			chatID:  chatID,
		},
	}
}

// GetDB returns the underlying *sql.DB
func (w *PostgresWrapper) GetDB() *sql.DB {
	sqlDB, err := w.db.DB()
	if err != nil {
		log.Printf("Failed to get SQL DB: %v", err)
		return nil
	}
	return sqlDB
}

// GetSchema fetches the current database schema
func (w *PostgresWrapper) GetSchema(ctx context.Context) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresWrapper -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	if err := w.updateUsage(); err != nil {
		return nil, fmt.Errorf("failed to update usage: %v", err)
	}

	driver, exists := w.manager.drivers["postgresql"]
	if !exists {
		// Check if yugabytedb driver exists
		driver, exists = w.manager.drivers["yugabytedb"]
		if !exists {
			return nil, fmt.Errorf("driver not found")
		}
	}

	if fetcher, ok := driver.(SchemaFetcher); ok {
		return fetcher.GetSchema(ctx, w)
	}
	return nil, fmt.Errorf("driver does not support schema fetching")
}

// GetTableChecksum calculates checksum for a single table
func (w *PostgresWrapper) GetTableChecksum(ctx context.Context, table string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("PostgresWrapper -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	if err := w.updateUsage(); err != nil {
		return "", fmt.Errorf("failed to update usage: %v", err)
	}

	driver, exists := w.manager.drivers["postgresql"]
	if !exists {
		// Check if yugabytedb driver exists
		driver, exists = w.manager.drivers["yugabytedb"]
		if !exists {
			return "", fmt.Errorf("driver not found")
		}
	}

	if fetcher, ok := driver.(SchemaFetcher); ok {
		return fetcher.GetTableChecksum(ctx, w, table)
	}
	return "", fmt.Errorf("driver does not support checksum calculation")
}

// Raw executes a raw SQL query
func (w *PostgresWrapper) Raw(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Error
}

// Exec executes a SQL statement
func (w *PostgresWrapper) Exec(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Exec(sql, values...).Error
}

// Query executes a SQL query and scans the result into dest
func (w *PostgresWrapper) Query(sql string, dest interface{}, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Scan(dest).Error
}

// Close closes the database connection
func (w *PostgresWrapper) Close() error {
	sqlDB, err := w.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Example of how to add support for another database type:
/*
type MySQLWrapper struct {
	BaseWrapper
	db *gorm.DB  // or other MySQL-specific client
}

func NewMySQLWrapper(db *gorm.DB, manager *Manager, chatID string) *MySQLWrapper {
	return &MySQLWrapper{
		BaseWrapper: BaseWrapper{
			manager: manager,
			chatID:  chatID,
		},
		db: db,
	}
}

// Implement DBExecutor interface for MySQL
func (w *MySQLWrapper) Raw(sql string, values ...interface{}) error {
	// MySQL-specific implementation
}

func (w *MySQLWrapper) Exec(sql string, values ...interface{}) error {
	// MySQL-specific implementation
}

func (w *MySQLWrapper) Query(sql string, dest interface{}, values ...interface{}) error {
	// MySQL-specific implementation
}

func (w *MySQLWrapper) Close() error {
	// MySQL-specific implementation
}
*/
