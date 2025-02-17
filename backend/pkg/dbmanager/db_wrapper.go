package dbmanager

import (
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
}

// BaseWrapper provides common functionality for all DB wrappers
type BaseWrapper struct {
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
	db *gorm.DB
}

func NewPostgresWrapper(db *gorm.DB, manager *Manager, chatID string) *PostgresWrapper {
	return &PostgresWrapper{
		BaseWrapper: BaseWrapper{
			manager: manager,
			chatID:  chatID,
		},
		db: db,
	}
}

func (w *PostgresWrapper) Raw(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Error
}

func (w *PostgresWrapper) Exec(sql string, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Exec(sql, values...).Error
}

func (w *PostgresWrapper) Query(sql string, dest interface{}, values ...interface{}) error {
	if err := w.updateUsage(); err != nil {
		return fmt.Errorf("failed to update usage: %v", err)
	}
	return w.db.Raw(sql, values...).Scan(dest).Error
}

func (w *PostgresWrapper) Close() error {
	sqlDB, err := w.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// DB returns the underlying GORM DB (PostgreSQL specific)
func (w *PostgresWrapper) DB() *gorm.DB {
	w.updateUsage()
	return w.db
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
