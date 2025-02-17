package dbmanager

import (
	"context"
	"crypto/md5"
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

// GetSchema fetches the current database schema
func (w *PostgresWrapper) GetSchema(ctx context.Context) (*SchemaInfo, error) {
	fetcher := &PostgresSchemaFetcher{db: w}
	return fetcher.FetchSchema(ctx)
}

// GetTableChecksum calculates checksum for a single table
func (w *PostgresWrapper) GetTableChecksum(ctx context.Context, table string) (string, error) {
	// Get table definition
	var tableDefinition string
	query := `
		SELECT 
			'CREATE TABLE ' || relname || E'\n(\n' ||
			array_to_string(
				array_agg(
					'    ' || column_name || ' ' ||  type || ' ' ||
					case when is_nullable = 'NO' then 'NOT NULL' else '' end ||
					case when column_default is not null then ' DEFAULT ' || column_default else '' end
				), E',\n'
			) || E'\n);\n' as definition
		FROM (
			SELECT 
				c.relname, a.attname AS column_name,
				pg_catalog.format_type(a.atttypid, a.atttypmod) as type,
				(SELECT substring(pg_catalog.pg_get_expr(d.adbin, d.adrelid) for 128)
				FROM pg_catalog.pg_attrdef d
				WHERE d.adrelid = a.attrelid AND d.adnum = a.attnum AND a.atthasdef) as column_default,
				n.nspname as schema,
				c.relname as table_name,
				a.attnum as column_position,
				case when a.attnotnull then 'NO' else 'YES' end as is_nullable
			FROM pg_catalog.pg_class c
			JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_catalog.pg_attribute a ON c.oid = a.attrelid
			WHERE n.nspname = 'public'
			AND c.relname = $1
			AND a.attnum > 0
			AND NOT a.attisdropped
			ORDER BY a.attnum
		) t
		GROUP BY relname;
	`

	err := w.Query(query, &tableDefinition, table)
	if err != nil {
		return "", fmt.Errorf("failed to get table definition: %v", err)
	}

	// Get indexes
	var indexes []string
	indexQuery := `
		SELECT indexdef
		FROM pg_indexes
		WHERE tablename = $1
		AND schemaname = 'public'
		ORDER BY indexname;
	`

	err = w.Query(indexQuery, &indexes, table)
	if err != nil {
		return "", fmt.Errorf("failed to get indexes: %v", err)
	}

	// Get foreign keys
	var foreignKeys []string
	fkQuery := `
		SELECT
			'ALTER TABLE ' || tc.table_name || ' ADD CONSTRAINT ' || tc.constraint_name ||
			' FOREIGN KEY (' || kcu.column_name || ') REFERENCES ' ||
			ccu.table_name || ' (' || ccu.column_name || ');'
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.table_name = $1
		AND tc.constraint_type = 'FOREIGN KEY';
	`

	err = w.Query(fkQuery, &foreignKeys, table)
	if err != nil {
		return "", fmt.Errorf("failed to get foreign keys: %v", err)
	}

	// Combine all definitions
	fullDefinition := tableDefinition
	for _, idx := range indexes {
		fullDefinition += idx + ";\n"
	}
	for _, fk := range foreignKeys {
		fullDefinition += fk + "\n"
	}

	// Calculate checksum
	return fmt.Sprintf("%x", md5.Sum([]byte(fullDefinition))), nil
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
