package dbmanager

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/pkg/redis"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"crypto/md5"
)

// Add these constants
const (
	schemaKeyPrefix = "schema:"
	schemaTTL       = 7 * 24 * time.Hour // Keep schemas for 7 days
)

// SchemaInfo represents database schema information
type SchemaInfo struct {
	Tables    map[string]TableSchema    `json:"tables"`
	Views     map[string]ViewSchema     `json:"views,omitempty"`
	Sequences map[string]SequenceSchema `json:"sequences,omitempty"`
	Enums     map[string]EnumSchema     `json:"enums,omitempty"`
	UpdatedAt time.Time                 `json:"updated_at"`
	Checksum  string                    `json:"checksum"`
}

type TableSchema struct {
	Name        string                    `json:"name"`
	Columns     map[string]ColumnInfo     `json:"columns"`
	Indexes     map[string]IndexInfo      `json:"indexes"`
	ForeignKeys map[string]ForeignKey     `json:"foreign_keys"`
	Constraints map[string]ConstraintInfo `json:"constraints"`
	Comment     string                    `json:"comment,omitempty"`
	Checksum    string                    `json:"checksum"`
	RowCount    int64                     `json:"row_count"`
}

type ColumnInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	IsNullable   bool   `json:"is_nullable"`
	DefaultValue string `json:"default_value,omitempty"`
	Comment      string `json:"comment,omitempty"`
}

type IndexInfo struct {
	Name     string   `json:"name"`
	Columns  []string `json:"columns"`
	IsUnique bool     `json:"is_unique"`
}

type ForeignKey struct {
	Name       string `json:"name"`
	ColumnName string `json:"column_name"`
	RefTable   string `json:"ref_table"`
	RefColumn  string `json:"ref_column"`
	OnDelete   string `json:"on_delete"`
	OnUpdate   string `json:"on_update"`
}

// SchemaDiff represents changes in schema
type SchemaDiff struct {
	AddedTables    []string             `json:"added_tables,omitempty"`
	RemovedTables  []string             `json:"removed_tables,omitempty"`
	ModifiedTables map[string]TableDiff `json:"modified_tables,omitempty"`
	UpdatedAt      time.Time            `json:"updated_at"`
	IsFirstTime    bool                 `json:"is_first_time,omitempty"`
	FullSchema     *SchemaInfo          `json:"full_schema,omitempty"`
}

type TableDiff struct {
	AddedColumns    []string `json:"added_columns,omitempty"`
	RemovedColumns  []string `json:"removed_columns,omitempty"`
	ModifiedColumns []string `json:"modified_columns,omitempty"`
	AddedIndexes    []string `json:"added_indexes,omitempty"`
	RemovedIndexes  []string `json:"removed_indexes,omitempty"`
	AddedFKs        []string `json:"added_fks,omitempty"`
	RemovedFKs      []string `json:"removed_fks,omitempty"`
}

// SchemaStorage handles efficient schema storage
type SchemaStorage struct {
	// Full schema with all details (for diffing and internal use)
	FullSchema *SchemaInfo `json:"full_schema"`

	// Simplified schema for LLM (only essential info)
	LLMSchema *LLMSchemaInfo `json:"llm_schema"`

	// Table-level checksums for quick change detection
	TableChecksums map[string]string `json:"table_checksums"`

	UpdatedAt time.Time `json:"updated_at"`
}

// LLMSchemaInfo is a simplified schema representation for the LLM
type LLMSchemaInfo struct {
	Tables map[string]LLMTableInfo `json:"tables"`
	// Include only what LLM needs to understand the schema
	Relationships []SchemaRelationship `json:"relationships"`
}

type LLMTableInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Columns     []LLMColumnInfo `json:"columns"`
	PrimaryKey  string          `json:"primary_key,omitempty"`
	RowCount    int64           `json:"row_count"`
}

type LLMColumnInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	IsNullable  bool   `json:"is_nullable"`
	IsIndexed   bool   `json:"is_indexed,omitempty"`
}

type SchemaRelationship struct {
	FromTable string `json:"from_table"`
	ToTable   string `json:"to_table"`
	Type      string `json:"type"`              // "one_to_one", "one_to_many", etc.
	Through   string `json:"through,omitempty"` // For many-to-many relationships
}

// Update the interfaces
type SchemaFetcher interface {
	GetSchema(ctx context.Context, db DBExecutor) (*SchemaInfo, error)
	GetTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error)
}

// Update SchemaManager struct
type SchemaManager struct {
	mu             sync.RWMutex
	schemaCache    map[string]*SchemaInfo
	storageService *SchemaStorageService
	dbManager      *Manager
	fetcherMap     map[string]func(DBExecutor) SchemaFetcher
}

func NewSchemaManager(redisRepo redis.IRedisRepositories, encryptionKey string, dbManager *Manager) (*SchemaManager, error) {
	storageService, err := NewSchemaStorageService(redisRepo, encryptionKey)
	if err != nil {
		return nil, err
	}

	manager := &SchemaManager{
		schemaCache:    make(map[string]*SchemaInfo),
		storageService: storageService,
		dbManager:      dbManager,
		fetcherMap:     make(map[string]func(DBExecutor) SchemaFetcher),
	}

	// Register default fetchers
	manager.registerDefaultFetchers()

	return manager, nil
}

func (sm *SchemaManager) SetDBManager(dbManager *Manager) {
	sm.dbManager = dbManager
}

// RegisterFetcher registers a new schema fetcher for a database type
func (sm *SchemaManager) RegisterFetcher(dbType string, constructor func(DBExecutor) SchemaFetcher) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.fetcherMap[dbType] = constructor
}

// getFetcher returns appropriate schema fetcher for the database type
func (sm *SchemaManager) getFetcher(dbType string, db DBExecutor) (SchemaFetcher, error) {
	sm.mu.RLock()
	constructor, exists := sm.fetcherMap[dbType]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no schema fetcher registered for database type: %s", dbType)
	}

	return constructor(db), nil
}

// Update schema fetching methods to use appropriate fetcher
func (sm *SchemaManager) fetchSchema(ctx context.Context, db DBExecutor, dbType string) (*SchemaInfo, error) {
	fetcher, err := sm.getFetcher(dbType, db)
	if err != nil {
		return nil, err
	}
	return fetcher.GetSchema(ctx, db)
}

// Update GetSchema to use fetchSchema and getFetcher
func (sm *SchemaManager) GetSchema(ctx context.Context, chatID string, db DBExecutor, dbType string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("GetSchema -> context cancelled: %v", err)
		return nil, err
	}

	// Always fetch fresh schema from database for schema checks
	schema, err := sm.fetchSchema(ctx, db, dbType)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schema: %v", err)
	}

	// Log all tables found in the database
	tableNames := make([]string, 0, len(schema.Tables))
	for tableName := range schema.Tables {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("GetSchema -> context cancelled: %v", err)
			return nil, err
		}
		tableNames = append(tableNames, tableName)
	}
	log.Printf("SchemaManager -> GetSchema -> Fresh schema contains tables: %v", tableNames)

	return schema, nil
}

// Fix the CheckSchemaChanges function to properly call CompareSchemas
func (sm *SchemaManager) CheckSchemaChanges(ctx context.Context, chatID string, db DBExecutor, dbType string) (*SchemaDiff, bool, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("CheckSchemaChanges -> context cancelled: %v", err)
		return nil, false, err
	}

	_, exists := sm.dbManager.drivers[dbType]
	if !exists {
		return nil, false, fmt.Errorf("no driver found for type: %s", dbType)
	}

	log.Printf("SchemaManager -> CheckSchemaChanges -> Getting current schema for chatID: %s", chatID)
	// Get current schema using driver
	currentSchema, err := sm.GetSchema(ctx, chatID, db, dbType)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get current schema: %v", err)
	}

	// Log current tables for debugging
	currentTables := make([]string, 0, len(currentSchema.Tables))
	for tableName := range currentSchema.Tables {
		currentTables = append(currentTables, tableName)
	}
	log.Printf("SchemaManager -> CheckSchemaChanges -> Current schema has tables: %v", currentTables)

	// Try to get stored schema
	storedSchema, err := sm.getStoredSchema(ctx, chatID)
	if err != nil {
		log.Printf("SchemaManager -> CheckSchemaChanges -> No stored schema found: %v", err)

		// First time - store current schema
		if err := sm.storeSchema(ctx, chatID, currentSchema, db, dbType); err != nil {
			return nil, true, err
		}

		// Return special diff for first time with full schema
		return &SchemaDiff{
			FullSchema:  currentSchema,
			UpdatedAt:   time.Now(),
			IsFirstTime: true,
		}, true, nil
	}

	// Log stored tables for debugging
	storedTables := make([]string, 0, len(storedSchema.FullSchema.Tables))
	for tableName := range storedSchema.FullSchema.Tables {
		storedTables = append(storedTables, tableName)
	}
	log.Printf("SchemaManager -> CheckSchemaChanges -> Stored schema has tables: %v", storedTables)

	// IMPORTANT: Use CompareSchemas (uppercase C) instead of compareSchemas (lowercase c)
	diff, hasChanges := sm.CompareSchemas(storedSchema.FullSchema, currentSchema)

	// Add detailed logging to see what's happening
	log.Printf("SchemaManager -> CheckSchemaChanges -> CompareSchemas returned hasChanges=%v, diff=%+v",
		hasChanges, diff)

	if hasChanges {
		log.Printf("SchemaManager -> CheckSchemaChanges -> Changes detected: added=%d, removed=%d, modified=%d",
			len(diff.AddedTables), len(diff.RemovedTables), len(diff.ModifiedTables))
	} else {
		log.Printf("SchemaManager -> CheckSchemaChanges -> No changes detected in comparison")
	}

	// Store the updated schema AFTER checking for changes
	if err := sm.storeSchema(ctx, chatID, currentSchema, db, dbType); err != nil {
		log.Printf("SchemaManager -> CheckSchemaChanges -> Error storing schema: %v", err)
		// Continue anyway, don't fail the whole operation
	}

	if !hasChanges {
		return nil, false, nil
	}

	return diff, true, nil
}

// Mark the old compareSchemas function as deprecated
// DEPRECATED: Use CompareSchemas instead
func (sm *SchemaManager) compareSchemas(old, new *SchemaInfo) *SchemaDiff {
	log.Printf("WARNING: Using deprecated compareSchemas function. Use CompareSchemas instead")
	diff, _ := sm.CompareSchemas(old, new)
	return diff
}

// Update compareTableSchemas to properly compare all table details
func compareTableSchemas(old, new TableSchema) TableDiff {
	diff := TableDiff{
		AddedColumns:    make([]string, 0),
		RemovedColumns:  make([]string, 0),
		ModifiedColumns: make([]string, 0),
		AddedIndexes:    make([]string, 0),
		RemovedIndexes:  make([]string, 0),
		AddedFKs:        make([]string, 0),
		RemovedFKs:      make([]string, 0),
	}

	// Compare columns
	for colName, newCol := range new.Columns {
		if oldCol, exists := old.Columns[colName]; !exists {
			log.Printf("compareTableSchemas -> Added column detected: %s", colName)
			diff.AddedColumns = append(diff.AddedColumns, colName)
		} else if !compareColumns(oldCol, newCol) {
			log.Printf("compareTableSchemas -> Modified column detected: %s (type: %s->%s, nullable: %v->%v, default: %s->%s)",
				colName, oldCol.Type, newCol.Type, oldCol.IsNullable, newCol.IsNullable,
				oldCol.DefaultValue, newCol.DefaultValue)
			diff.ModifiedColumns = append(diff.ModifiedColumns, colName)
		}
	}

	// Check for removed columns
	for colName := range old.Columns {
		if _, exists := new.Columns[colName]; !exists {
			log.Printf("compareTableSchemas -> Removed column detected: %s", colName)
			diff.RemovedColumns = append(diff.RemovedColumns, colName)
		}
	}

	// Compare indexes
	for idxName, newIdx := range new.Indexes {
		if oldIdx, exists := old.Indexes[idxName]; !exists {
			log.Printf("compareTableSchemas -> Added index detected: %s", idxName)
			diff.AddedIndexes = append(diff.AddedIndexes, idxName)
		} else if !reflect.DeepEqual(oldIdx, newIdx) {
			log.Printf("compareTableSchemas -> Modified index detected: %s", idxName)
			diff.RemovedIndexes = append(diff.RemovedIndexes, idxName)
			diff.AddedIndexes = append(diff.AddedIndexes, idxName)
		}
	}

	for idxName := range old.Indexes {
		if _, exists := new.Indexes[idxName]; !exists {
			log.Printf("compareTableSchemas -> Removed index detected: %s", idxName)
			diff.RemovedIndexes = append(diff.RemovedIndexes, idxName)
		}
	}

	// Compare foreign keys
	for fkName, newFK := range new.ForeignKeys {
		if oldFK, exists := old.ForeignKeys[fkName]; !exists {
			log.Printf("compareTableSchemas -> Added foreign key detected: %s", fkName)
			diff.AddedFKs = append(diff.AddedFKs, fkName)
		} else if !reflect.DeepEqual(oldFK, newFK) {
			log.Printf("compareTableSchemas -> Modified foreign key detected: %s", fkName)
			diff.RemovedFKs = append(diff.RemovedFKs, fkName)
			diff.AddedFKs = append(diff.AddedFKs, fkName)
		}
	}

	for fkName := range old.ForeignKeys {
		if _, exists := new.ForeignKeys[fkName]; !exists {
			log.Printf("compareTableSchemas -> Removed foreign key detected: %s", fkName)
			diff.RemovedFKs = append(diff.RemovedFKs, fkName)
		}
	}

	return diff
}

// Fix the compareColumns function to be more thorough
func compareColumns(oldCol, newCol ColumnInfo) bool {
	// Log detailed comparison
	log.Printf("compareColumns -> Comparing column: name=%s", oldCol.Name)
	log.Printf("compareColumns -> Old column: type=%s, nullable=%v, default=%s, comment=%s",
		oldCol.Type, oldCol.IsNullable, oldCol.DefaultValue, oldCol.Comment)
	log.Printf("compareColumns -> New column: type=%s, nullable=%v, default=%s, comment=%s",
		newCol.Type, newCol.IsNullable, newCol.DefaultValue, newCol.Comment)

	// Compare all relevant column properties
	typeMatch := oldCol.Type == newCol.Type
	nullableMatch := oldCol.IsNullable == newCol.IsNullable
	defaultMatch := oldCol.DefaultValue == newCol.DefaultValue
	commentMatch := oldCol.Comment == newCol.Comment

	log.Printf("compareColumns -> Comparison results: typeMatch=%v, nullableMatch=%v, defaultMatch=%v, commentMatch=%v",
		typeMatch, nullableMatch, defaultMatch, commentMatch)

	return typeMatch && nullableMatch && defaultMatch && commentMatch
}

// Add the missing isEmpty method for TableDiff
func (td TableDiff) isEmpty() bool {
	return len(td.AddedColumns) == 0 &&
		len(td.RemovedColumns) == 0 &&
		len(td.ModifiedColumns) == 0 &&
		len(td.AddedIndexes) == 0 &&
		len(td.RemovedIndexes) == 0 &&
		len(td.AddedFKs) == 0 &&
		len(td.RemovedFKs) == 0
}

// Update storeSchema to properly set checksums
func (sm *SchemaManager) storeSchema(ctx context.Context, chatID string, schema *SchemaInfo, db DBExecutor, dbType string) error {
	// Get table checksums first
	checksums, err := sm.getTableChecksums(ctx, db, dbType)
	if err != nil {
		return fmt.Errorf("failed to get table checksums: %v", err)
	}

	// Set checksums in schema tables
	for tableName, checksum := range checksums {
		if table, exists := schema.Tables[tableName]; exists {
			table.Checksum = checksum
			// Need to reassign since table is a copy
			schema.Tables[tableName] = table
		}
	}

	// Update in-memory cache with the updated schema
	sm.mu.Lock()
	sm.schemaCache[chatID] = schema
	sm.mu.Unlock()

	// Create storage object
	storage := &SchemaStorage{
		FullSchema:     schema,
		LLMSchema:      sm.createLLMSchema(schema, dbType),
		TableChecksums: checksums,
		UpdatedAt:      time.Now(),
	}

	log.Printf("SchemaManager -> storeSchema -> Storing schema with checksums: %+v", checksums)

	// Store in Redis
	if err := sm.storageService.Store(ctx, chatID, storage); err != nil {
		return fmt.Errorf("failed to store schema in Redis: %v", err)
	}

	return nil
}

// Get current table checksums without fetching full schema
func (sm *SchemaManager) getTableChecksums(ctx context.Context, db DBExecutor, dbType string) (map[string]string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("getTableChecksums -> context cancelled: %v", err)
		return nil, err
	}

	switch dbType {
	case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("getTableChecksums -> context cancelled: %v", err)
			return nil, err
		}

		checksums := make(map[string]string)

		// Get schema directly from the database
		schema, err := db.GetSchema(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get schema: %v", err)
		}

		// Calculate checksums for each table
		for tableName, table := range schema.Tables {
			// Check for context cancellation
			if err := ctx.Err(); err != nil {
				log.Printf("getTableChecksums -> context cancelled: %v", err)
				return nil, err
			}

			// Convert table definition to string for checksum
			tableStr := fmt.Sprintf("%s:%v:%v:%v:%v",
				tableName,
				table.Columns,
				table.Indexes,
				table.ForeignKeys,
				table.Constraints,
			)

			// Calculate checksum using crypto/md5
			hasher := md5.New()
			hasher.Write([]byte(tableStr))
			checksum := hex.EncodeToString(hasher.Sum(nil))
			checksums[tableName] = checksum
		}
		return checksums, nil
	case constants.DatabaseTypeMySQL:
		// TODO: Implement MySQL checksum calculation
		checksums := make(map[string]string)
		return checksums, nil
	}

	return nil, fmt.Errorf("unsupported database type: %s", dbType)
}

// Update fetchTableList to use driver directly
func (sm *SchemaManager) fetchTableList(ctx context.Context, db DBExecutor) ([]string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("fetchTableList -> context cancelled: %v", err)
		return nil, err
	}

	schema, err := db.GetSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %v", err)
	}

	tables := make([]string, 0, len(schema.Tables))
	for tableName := range schema.Tables {
		tables = append(tables, tableName)
	}
	sort.Strings(tables) // Ensure consistent order
	return tables, nil
}

// Update schema retrieval methods
func (sm *SchemaManager) getStoredSchema(ctx context.Context, chatID string) (*SchemaStorage, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("getStoredSchema -> context cancelled: %v", err)
		return nil, err
	}

	return sm.storageService.Retrieve(ctx, chatID)
}

// Add type-specific schema simplification
type SchemaSimplifier interface {
	SimplifyDataType(dbType string) string
	GetColumnConstraints(col ColumnInfo, table TableSchema) []string
}

// Add database-specific simplifiers
type PostgresSimplifier struct{}

func (s *PostgresSimplifier) SimplifyDataType(dbType string) string {
	switch strings.ToLower(dbType) {
	case "integer", "bigint", "smallint":
		return "number"
	case "character varying", "text", "char", "varchar":
		return "text"
	case "boolean":
		return "boolean"
	case "timestamp without time zone", "timestamp with time zone":
		return "timestamp"
	case "date":
		return "date"
	case "numeric", "decimal", "real", "double precision":
		return "decimal"
	case "jsonb", "json":
		return "json"
	default:
		return dbType
	}
}

func (s *PostgresSimplifier) GetColumnConstraints(col ColumnInfo, table TableSchema) []string {
	constraints := make([]string, 0)

	if !col.IsNullable {
		constraints = append(constraints, "NOT NULL")
	}

	if col.DefaultValue != "" {
		constraints = append(constraints, fmt.Sprintf("DEFAULT %s", col.DefaultValue))
	}

	// Check if column is part of any unique index
	for _, idx := range table.Indexes {
		if idx.IsUnique && len(idx.Columns) == 1 && idx.Columns[0] == col.Name {
			constraints = append(constraints, "UNIQUE")
			break
		}
	}

	// Check if column is a foreign key
	for _, fk := range table.ForeignKeys {
		if fk.ColumnName == col.Name {
			constraints = append(constraints, fmt.Sprintf("REFERENCES %s(%s)", fk.RefTable, fk.RefColumn))
			break
		}
	}

	return constraints
}

type MySQLSimplifier struct{}

func (s *MySQLSimplifier) SimplifyDataType(dbType string) string {
	switch strings.ToLower(dbType) {
	case "int", "bigint", "tinyint", "smallint":
		return "number"
	case "varchar", "text", "char":
		return "text"
	default:
		return dbType
	}
}

func (s *MySQLSimplifier) GetColumnConstraints(col ColumnInfo, table TableSchema) []string {
	constraints := make([]string, 0)

	if !col.IsNullable {
		constraints = append(constraints, "NOT NULL")
	}

	if col.DefaultValue != "" {
		constraints = append(constraints, fmt.Sprintf("DEFAULT %s", col.DefaultValue))
	}

	// Check if column is part of any unique index
	for _, idx := range table.Indexes {
		if idx.IsUnique && len(idx.Columns) == 1 && idx.Columns[0] == col.Name {
			constraints = append(constraints, "UNIQUE")
			break
		}
	}

	// Check if column is a foreign key
	for _, fk := range table.ForeignKeys {
		if fk.ColumnName == col.Name {
			constraints = append(constraints, fmt.Sprintf("FOREIGN KEY REFERENCES %s(%s)", fk.RefTable, fk.RefColumn))
			break
		}
	}

	return constraints
}

func (sm *SchemaManager) isColumnIndexed(colName string, indexes map[string]IndexInfo) bool {
	for _, idx := range indexes {
		for _, col := range idx.Columns {
			if col == colName {
				return true
			}
		}
	}
	return false
}

func (sm *SchemaManager) isColumnUnique(tableName, colName string, schema *SchemaInfo) bool {
	table, exists := schema.Tables[tableName]
	if !exists {
		return false
	}

	for _, idx := range table.Indexes {
		if idx.IsUnique && len(idx.Columns) == 1 && idx.Columns[0] == colName {
			return true
		}
	}
	return false
}

// Ensure both simplifiers implement the interface
var (
	_ SchemaSimplifier = (*PostgresSimplifier)(nil)
	_ SchemaSimplifier = (*MySQLSimplifier)(nil)
)

// FormatSchemaForLLM formats the schema into a LLM-friendly string
func (m *SchemaManager) FormatSchemaForLLM(schema *SchemaInfo) string {
	var result strings.Builder
	result.WriteString("Current Database Schema:\n\n")

	// Sort tables for consistent output
	tableNames := make([]string, 0, len(schema.Tables))
	for tableName := range schema.Tables {
		tableNames = append(tableNames, tableName)
	}
	sort.Strings(tableNames)

	// Format schema for LLM for tables, columns, indexes, foreign keys, constraints, etc.
	for _, tableName := range tableNames {
		table := schema.Tables[tableName]
		result.WriteString(fmt.Sprintf("Table: %s\n", tableName))
		if table.Comment != "" {
			result.WriteString(fmt.Sprintf("Description: %s\n", table.Comment))
		}

		// Sort columns for consistent output
		columnNames := make([]string, 0, len(table.Columns))
		for columnName := range table.Columns {
			columnNames = append(columnNames, columnName)
		}
		sort.Strings(columnNames)

		for _, columnName := range columnNames {
			column := table.Columns[columnName]
			nullable := "NOT NULL"
			if column.IsNullable {
				nullable = "NULL"
			}
			result.WriteString(fmt.Sprintf("  - %s (%s) %s",
				columnName,
				column.Type,
				nullable,
			))

			// Check if column is primary key by looking at indexes
			for _, idx := range table.Indexes {
				if len(idx.Columns) == 1 && idx.Columns[0] == columnName {
					result.WriteString(" PRIMARY KEY")
					break
				}
			}

			if column.DefaultValue != "" {
				result.WriteString(fmt.Sprintf(" DEFAULT %s", column.DefaultValue))
			}
			if column.Comment != "" {
				result.WriteString(fmt.Sprintf(" -- %s", column.Comment))
			}
			result.WriteString("\n")
		}

		// Add foreign key information
		if len(table.ForeignKeys) > 0 {
			result.WriteString("\n  Foreign Keys:\n")
			for _, fk := range table.ForeignKeys {
				result.WriteString(fmt.Sprintf("  - %s -> %s.%s",
					fk.ColumnName,
					fk.RefTable,
					fk.RefColumn,
				))
				if fk.OnDelete != "" {
					result.WriteString(fmt.Sprintf(" ON DELETE %s", fk.OnDelete))
				}
				if fk.OnUpdate != "" {
					result.WriteString(fmt.Sprintf(" ON UPDATE %s", fk.OnUpdate))
				}
				result.WriteString("\n")
			}
		}
		result.WriteString("\n")

		// Add Enums information
		if len(schema.Enums) > 0 {
			result.WriteString("\n  Enums:\n")
			for _, enum := range schema.Enums {
				result.WriteString(fmt.Sprintf("  - %s", enum.Name))
			}
		}
		// Add indexes information
		if len(table.Indexes) > 0 {
			result.WriteString("\n  Indexes:\n")
			for _, idx := range table.Indexes {
				result.WriteString(fmt.Sprintf("  - %s (%s)", idx.Name, strings.Join(idx.Columns, ", ")))
			}
		}

		result.WriteString("\n")
		// Add constraints information
		if len(table.Constraints) > 0 {
			result.WriteString("\n  Constraints:\n")
			for _, constraint := range table.Constraints {
				result.WriteString(fmt.Sprintf("  - %s", constraint.Name))
			}
		}
		result.WriteString("\n")
		// Add Views information
		if len(schema.Views) > 0 {
			result.WriteString("\n  Views:\n")
			for _, view := range schema.Views {
				result.WriteString(fmt.Sprintf("  - %s", view.Name))
			}
		}
	}

	return result.String()
}

// HasSchemaChanged to support context cancellation
func (sm *SchemaManager) HasSchemaChanged(ctx context.Context, chatID string, db DBExecutor) (bool, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("HasSchemaChanged -> context cancelled: %v", err)
		return false, err
	}

	log.Printf("HasSchemaChanged -> chatID: %s", chatID)

	conn, exists := sm.dbManager.connections[chatID]
	if !exists {
		return true, fmt.Errorf("connection not found")
	}

	// Get current checksums first
	currentChecksums, err := sm.getTableChecksums(ctx, db, conn.Config.Type)
	if err != nil {
		log.Printf("HasSchemaChanged -> error getting current checksums: %v", err)
		return true, nil
	}

	// Check for context cancellation after expensive operation
	if err := ctx.Err(); err != nil {
		log.Printf("HasSchemaChanged -> context cancelled: %v", err)
		return false, err
	}

	log.Printf("HasSchemaChanged -> currentChecksums: %+v", currentChecksums)

	// Check in-memory cache
	sm.mu.RLock()
	cachedSchema, exists := sm.schemaCache[chatID]
	sm.mu.RUnlock()

	if exists && cachedSchema != nil {
		log.Printf("HasSchemaChanged -> using cached schema")

		// Compare table counts
		if len(cachedSchema.Tables) != len(currentChecksums) {
			log.Printf("HasSchemaChanged -> table count mismatch: cached=%d, current=%d",
				len(cachedSchema.Tables), len(currentChecksums))
			return true, nil
		}

		// Compare each table's checksum
		for tableName, currentChecksum := range currentChecksums {
			// Check for context cancellation during iteration
			if err := ctx.Err(); err != nil {
				log.Printf("HasSchemaChanged -> context cancelled: %v", err)
				return false, err
			}

			table, ok := cachedSchema.Tables[tableName]
			if !ok {
				log.Printf("HasSchemaChanged -> table %s not found in cache", tableName)
				return true, nil
			}

			log.Printf("HasSchemaChanged -> comparing table %s: cached=%s, current=%s",
				tableName, table.Checksum, currentChecksum)

			if table.Checksum != currentChecksum {
				log.Printf("HasSchemaChanged -> checksum mismatch for table %s", tableName)
				return true, nil
			}
		}
		return false, nil
	}

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("HasSchemaChanged -> context cancelled: %v", err)
		return false, err
	}

	// Check Redis if not in memory
	storage, err := sm.storageService.Retrieve(ctx, chatID)
	if err != nil || storage == nil {
		log.Printf("HasSchemaChanged -> no valid storage found: %v", err)
		return true, nil
	}

	return !reflect.DeepEqual(storage.TableChecksums, currentChecksums), nil
}

// Add TriggerType enum
type TriggerType string

const (
	TriggerTypeManual TriggerType = "manual" // For DDL operations
	TriggerTypeAuto   TriggerType = "auto"   // For interval checks
)

// Helper function to get the latest schema
func (sm *SchemaManager) GetLatestSchema(ctx context.Context, chatID string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("GetLatestSchema -> context cancelled: %v", err)
		return nil, err
	}

	// Get current connection
	db, err := sm.dbManager.GetConnection(chatID)
	if err != nil {
		log.Printf("GetLatestSchema -> error getting connection: %v", err)
		return nil, fmt.Errorf("failed to get connection: %v", err)
	}

	// Get connection config
	conn, exists := sm.dbManager.connections[chatID]
	if !exists {
		return nil, fmt.Errorf("connection not found for chatID: %s", chatID)
	}

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("GetLatestSchema -> context cancelled: %v", err)
		return nil, err
	}

	// For manual triggers (DDL), directly fetch and store new schema
	log.Printf("SchemaManager -> RefreshSchema -> Manual trigger, fetching new schema")
	schema, err := db.GetSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current schema: %v", err)
	}

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("GetLatestSchema -> context cancelled: %v", err)
		return nil, err
	}

	log.Printf("SchemaManager -> RefreshSchema -> Storing fresh schema")
	// Store the fresh schema immediately
	if err := sm.storeSchema(ctx, chatID, schema, db, conn.Config.Type); err != nil {
		return nil, fmt.Errorf("failed to store schema: %v", err)
	}

	return schema, nil
}

// Add new schema types
type ViewSchema struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

type SequenceSchema struct {
	Name       string `json:"name"`
	StartValue int64  `json:"start_value"`
	Increment  int64  `json:"increment"`
	MinValue   int64  `json:"min_value"`
	MaxValue   int64  `json:"max_value"`
	CacheSize  int64  `json:"cache_size"`
	IsCycled   bool   `json:"is_cycled"`
}

type EnumSchema struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
	Schema string   `json:"schema"`
}

type ConstraintInfo struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Definition string   `json:"definition,omitempty"`
	Columns    []string `json:"columns,omitempty"`
}

// Create LLM-friendly schema
func (sm *SchemaManager) createLLMSchema(schema *SchemaInfo, dbType string) *LLMSchemaInfo {
	var simplifier SchemaSimplifier
	switch dbType {
	case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
		simplifier = &PostgresSimplifier{}
	case constants.DatabaseTypeMySQL:
		simplifier = &MySQLSimplifier{}
	default:
		simplifier = &PostgresSimplifier{} // Default to PostgreSQL
	}

	llmSchema := &LLMSchemaInfo{
		Tables:        make(map[string]LLMTableInfo),
		Relationships: make([]SchemaRelationship, 0),
	}

	// Convert tables to LLM-friendly format
	for tableName, table := range schema.Tables {
		llmTable := LLMTableInfo{
			Name:        tableName,
			Description: table.Comment,
			Columns:     make([]LLMColumnInfo, 0),
			RowCount:    table.RowCount,
		}

		// Convert columns with simplified types
		for _, col := range table.Columns {
			llmCol := LLMColumnInfo{
				Name:        col.Name,
				Type:        simplifier.SimplifyDataType(col.Type),
				Description: col.Comment,
				IsNullable:  col.IsNullable,
				IsIndexed:   sm.isColumnIndexed(col.Name, table.Indexes),
			}
			llmTable.Columns = append(llmTable.Columns, llmCol)
		}

		llmSchema.Tables[tableName] = llmTable
	}

	// Extract relationships
	llmSchema.Relationships = sm.extractRelationships(schema)

	return llmSchema
}

// Extract relationships from foreign keys
func (sm *SchemaManager) extractRelationships(schema *SchemaInfo) []SchemaRelationship {
	relationships := make([]SchemaRelationship, 0)
	processedPairs := make(map[string]bool)

	for tableName, table := range schema.Tables {
		for _, fk := range table.ForeignKeys {
			// Create unique pair key to avoid duplicates
			pairKey := fmt.Sprintf("%s:%s", tableName, fk.RefTable)
			if processedPairs[pairKey] {
				continue
			}

			rel := SchemaRelationship{
				FromTable: tableName,
				ToTable:   fk.RefTable,
				Type:      sm.determineRelationType(schema, tableName, fk),
			}
			relationships = append(relationships, rel)
			processedPairs[pairKey] = true
		}
	}

	return relationships
}

// Determine relationship type (one-to-one, one-to-many, etc.)
func (sm *SchemaManager) determineRelationType(schema *SchemaInfo, fromTable string, fk ForeignKey) string {
	// Check if the foreign key column is unique
	if sm.isColumnUnique(fromTable, fk.ColumnName, schema) {
		return "one_to_one"
	}
	return "one_to_many"
}

// Compress schema for storage
func (sm *SchemaManager) compressSchema(storage *SchemaStorage) ([]byte, error) {
	// Marshal to JSON first
	data, err := json.Marshal(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %v", err)
	}

	// Use zlib compression
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to compress data: %v", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close compressor: %v", err)
	}

	return buf.Bytes(), nil
}

// Add method to register default fetchers
func (sm *SchemaManager) registerDefaultFetchers() {
	// Register PostgreSQL fetcher
	sm.RegisterFetcher("postgresql", func(db DBExecutor) SchemaFetcher {
		return &PostgresDriver{}
	})

	sm.RegisterFetcher("yugabytedb", func(db DBExecutor) SchemaFetcher {
		return &PostgresDriver{}
	})

	// Register MySQL fetcher when implemented
	// sm.RegisterFetcher("mysql", func(db DBExecutor) SchemaFetcher {
	//     return &MySQLDriver{}
	// })
}

// Update the CompareSchemasDetailed function to be more precise
func (sm *SchemaManager) CompareSchemasDetailed(oldSchema, newSchema *SchemaInfo) (*SchemaDiff, bool) {
	diff := &SchemaDiff{
		AddedTables:    make([]string, 0),
		RemovedTables:  make([]string, 0),
		ModifiedTables: make(map[string]TableDiff),
		UpdatedAt:      time.Now(),
	}

	hasChanges := false

	// Check for added/removed tables
	for tableName := range newSchema.Tables {
		if _, exists := oldSchema.Tables[tableName]; !exists {
			diff.AddedTables = append(diff.AddedTables, tableName)
			hasChanges = true
		}
	}

	for tableName := range oldSchema.Tables {
		if _, exists := newSchema.Tables[tableName]; !exists {
			diff.RemovedTables = append(diff.RemovedTables, tableName)
			hasChanges = true
		}
	}

	// Check for modified tables
	for tableName, newTable := range newSchema.Tables {
		oldTable, exists := oldSchema.Tables[tableName]
		if !exists {
			continue // Already handled as added table
		}

		// Skip if table checksums match exactly
		if oldTable.Checksum == newTable.Checksum {
			continue
		}

		tableDiff := TableDiff{
			AddedColumns:    make([]string, 0),
			RemovedColumns:  make([]string, 0),
			ModifiedColumns: make([]string, 0),
			AddedIndexes:    make([]string, 0),
			RemovedIndexes:  make([]string, 0),
			AddedFKs:        make([]string, 0),
			RemovedFKs:      make([]string, 0),
		}

		// Check columns
		for colName := range newTable.Columns {
			if _, exists := oldTable.Columns[colName]; !exists {
				tableDiff.AddedColumns = append(tableDiff.AddedColumns, colName)
				hasChanges = true
			}
		}

		for colName, oldCol := range oldTable.Columns {
			newCol, exists := newTable.Columns[colName]
			if !exists {
				tableDiff.RemovedColumns = append(tableDiff.RemovedColumns, colName)
				hasChanges = true
				continue
			}

			// Check if column definition changed
			if !reflect.DeepEqual(oldCol, newCol) {
				tableDiff.ModifiedColumns = append(tableDiff.ModifiedColumns, colName)
				hasChanges = true
			}
		}

		// Check indexes
		for idxName := range newTable.Indexes {
			if _, exists := oldTable.Indexes[idxName]; !exists {
				tableDiff.AddedIndexes = append(tableDiff.AddedIndexes, idxName)
				hasChanges = true
			}
		}

		for idxName, oldIdx := range oldTable.Indexes {
			newIdx, exists := newTable.Indexes[idxName]
			if !exists {
				tableDiff.RemovedIndexes = append(tableDiff.RemovedIndexes, idxName)
				hasChanges = true
				continue
			}

			// Check if index definition changed
			if !reflect.DeepEqual(oldIdx, newIdx) {
				// Consider this a removed and added index
				tableDiff.RemovedIndexes = append(tableDiff.RemovedIndexes, idxName)
				tableDiff.AddedIndexes = append(tableDiff.AddedIndexes, idxName)
				hasChanges = true
			}
		}

		// Check foreign keys
		for fkName := range newTable.ForeignKeys {
			if _, exists := oldTable.ForeignKeys[fkName]; !exists {
				tableDiff.AddedFKs = append(tableDiff.AddedFKs, fkName)
				hasChanges = true
			}
		}

		for fkName, oldFK := range oldTable.ForeignKeys {
			newFK, exists := newTable.ForeignKeys[fkName]
			if !exists {
				tableDiff.RemovedFKs = append(tableDiff.RemovedFKs, fkName)
				hasChanges = true
				continue
			}

			// Check if FK definition changed
			if !reflect.DeepEqual(oldFK, newFK) {
				// Consider this a removed and added FK
				tableDiff.RemovedFKs = append(tableDiff.RemovedFKs, fkName)
				tableDiff.AddedFKs = append(tableDiff.AddedFKs, fkName)
				hasChanges = true
			}
		}

		// Only add table diff if there are actual changes
		if len(tableDiff.AddedColumns) > 0 || len(tableDiff.RemovedColumns) > 0 ||
			len(tableDiff.ModifiedColumns) > 0 || len(tableDiff.AddedIndexes) > 0 ||
			len(tableDiff.RemovedIndexes) > 0 || len(tableDiff.AddedFKs) > 0 ||
			len(tableDiff.RemovedFKs) > 0 {
			diff.ModifiedTables[tableName] = tableDiff
		}
	}

	// If no changes were detected, return nil diff
	if !hasChanges {
		return nil, false
	}

	return diff, true
}

// Fix the CompareSchemas function to always do detailed comparison
func (sm *SchemaManager) CompareSchemas(oldSchema, newSchema *SchemaInfo) (*SchemaDiff, bool) {
	// Skip the checksum comparison and always do detailed comparison
	log.Printf("SchemaManager -> CompareSchemas -> Performing detailed comparison regardless of checksums")

	diff := &SchemaDiff{
		AddedTables:    make([]string, 0),
		RemovedTables:  make([]string, 0),
		ModifiedTables: make(map[string]TableDiff),
		UpdatedAt:      time.Now(),
	}

	hasChanges := false

	// Log table counts for debugging
	log.Printf("SchemaManager -> CompareSchemas -> Old schema has %d tables, new schema has %d tables",
		len(oldSchema.Tables), len(newSchema.Tables))

	// Log table names for better debugging
	oldTableNames := make([]string, 0, len(oldSchema.Tables))
	for tableName := range oldSchema.Tables {
		oldTableNames = append(oldTableNames, tableName)
	}

	newTableNames := make([]string, 0, len(newSchema.Tables))
	for tableName := range newSchema.Tables {
		newTableNames = append(newTableNames, tableName)
	}

	log.Printf("SchemaManager -> CompareSchemas -> Old tables: %v", oldTableNames)
	log.Printf("SchemaManager -> CompareSchemas -> New tables: %v", newTableNames)

	// Check for added tables
	for tableName := range newSchema.Tables {
		_, exists := oldSchema.Tables[tableName]
		log.Printf("SchemaManager -> CompareSchemas -> Checking if table %s exists in old schema: %v", tableName, exists)

		if !exists {
			log.Printf("SchemaManager -> CompareSchemas -> Added table detected: %s", tableName)
			diff.AddedTables = append(diff.AddedTables, tableName)
			hasChanges = true
		}
	}

	// Check for removed tables
	for tableName := range oldSchema.Tables {
		_, exists := newSchema.Tables[tableName]
		log.Printf("SchemaManager -> CompareSchemas -> Checking if table %s exists in new schema: %v", tableName, exists)

		if !exists {
			log.Printf("SchemaManager -> CompareSchemas -> Removed table detected: %s", tableName)
			diff.RemovedTables = append(diff.RemovedTables, tableName)
			hasChanges = true
		}
	}

	// Check for modified tables - CRITICAL FIX
	for tableName, newTable := range newSchema.Tables {
		oldTable, exists := oldSchema.Tables[tableName]
		if !exists {
			continue // Already handled as added table
		}

		// Always perform detailed comparison using compareTableSchemas
		tableDiff := compareTableSchemas(oldTable, newTable)

		// Log the detailed comparison results
		log.Printf("SchemaManager -> CompareSchemas -> Table %s detailed comparison: added cols=%d, removed cols=%d, modified cols=%d, added indexes=%d, removed indexes=%d",
			tableName, len(tableDiff.AddedColumns), len(tableDiff.RemovedColumns),
			len(tableDiff.ModifiedColumns), len(tableDiff.AddedIndexes), len(tableDiff.RemovedIndexes))

		// Only add table diff if there are actual changes
		if !tableDiff.isEmpty() {
			diff.ModifiedTables[tableName] = tableDiff
			hasChanges = true
			log.Printf("SchemaManager -> CompareSchemas -> Table %s has changes", tableName)
		} else {
			log.Printf("SchemaManager -> CompareSchemas -> Table %s has no changes", tableName)
		}
	}

	// If no changes were detected, return false
	if !hasChanges {
		log.Printf("SchemaManager -> CompareSchemas -> No actual changes detected")
		return nil, false
	}

	log.Printf("SchemaManager -> CompareSchemas -> Changes detected: added tables=%d, removed tables=%d, modified tables=%d",
		len(diff.AddedTables), len(diff.RemovedTables), len(diff.ModifiedTables))
	return diff, true
}

// Improve the compareTableDetails method to detect all changes
func (sm *SchemaManager) compareTableDetails(oldTable, newTable *TableSchema) TableDiff {
	tableDiff := TableDiff{
		AddedColumns:    make([]string, 0),
		RemovedColumns:  make([]string, 0),
		ModifiedColumns: make([]string, 0),
		AddedIndexes:    make([]string, 0),
		RemovedIndexes:  make([]string, 0),
		AddedFKs:        make([]string, 0),
		RemovedFKs:      make([]string, 0),
	}

	// Check columns
	for colName, newCol := range newTable.Columns {
		if oldCol, exists := oldTable.Columns[colName]; !exists {
			log.Printf("SchemaManager -> compareTableDetails -> Added column detected: %s", colName)
			tableDiff.AddedColumns = append(tableDiff.AddedColumns, colName)
		} else {
			// Compare column details
			if newCol.Type != oldCol.Type ||
				newCol.IsNullable != oldCol.IsNullable ||
				newCol.DefaultValue != oldCol.DefaultValue {
				log.Printf("SchemaManager -> compareTableDetails -> Modified column detected: %s (type: %s->%s, nullable: %v->%v, default: %s->%s)",
					colName, oldCol.Type, newCol.Type, oldCol.IsNullable, newCol.IsNullable,
					oldCol.DefaultValue, newCol.DefaultValue)
				tableDiff.ModifiedColumns = append(tableDiff.ModifiedColumns, colName)
			}
		}
	}

	for colName := range oldTable.Columns {
		if _, exists := newTable.Columns[colName]; !exists {
			log.Printf("SchemaManager -> compareTableDetails -> Removed column detected: %s", colName)
			tableDiff.RemovedColumns = append(tableDiff.RemovedColumns, colName)
		}
	}

	// Check indexes
	for idxName, newIdx := range newTable.Indexes {
		if oldIdx, exists := oldTable.Indexes[idxName]; !exists {
			log.Printf("SchemaManager -> compareTableDetails -> Added index detected: %s", idxName)
			tableDiff.AddedIndexes = append(tableDiff.AddedIndexes, idxName)
		} else {
			// Compare index details
			if !reflect.DeepEqual(oldIdx, newIdx) {
				log.Printf("SchemaManager -> compareTableDetails -> Modified index detected: %s", idxName)
				tableDiff.RemovedIndexes = append(tableDiff.RemovedIndexes, idxName)
				tableDiff.AddedIndexes = append(tableDiff.AddedIndexes, idxName)
			}
		}
	}

	for idxName := range oldTable.Indexes {
		if _, exists := newTable.Indexes[idxName]; !exists {
			log.Printf("SchemaManager -> compareTableDetails -> Removed index detected: %s", idxName)
			tableDiff.RemovedIndexes = append(tableDiff.RemovedIndexes, idxName)
		}
	}

	// Check foreign keys
	for fkName, newFK := range newTable.ForeignKeys {
		if oldFK, exists := oldTable.ForeignKeys[fkName]; !exists {
			log.Printf("SchemaManager -> compareTableDetails -> Added foreign key detected: %s", fkName)
			tableDiff.AddedFKs = append(tableDiff.AddedFKs, fkName)
		} else {
			// Compare FK details
			if !reflect.DeepEqual(oldFK, newFK) {
				log.Printf("SchemaManager -> compareTableDetails -> Modified foreign key detected: %s", fkName)
				tableDiff.RemovedFKs = append(tableDiff.RemovedFKs, fkName)
				tableDiff.AddedFKs = append(tableDiff.AddedFKs, fkName)
			}
		}
	}

	for fkName := range oldTable.ForeignKeys {
		if _, exists := newTable.ForeignKeys[fkName]; !exists {
			log.Printf("SchemaManager -> compareTableDetails -> Removed foreign key detected: %s", fkName)
			tableDiff.RemovedFKs = append(tableDiff.RemovedFKs, fkName)
		}
	}

	return tableDiff
}

// Add a method to clear schema cache
func (sm *SchemaManager) ClearSchemaCache(chatID string) {
	sm.mu.Lock()
	delete(sm.schemaCache, chatID)
	sm.mu.Unlock()
	log.Printf("SchemaManager -> ClearSchemaCache -> Cleared schema cache for chatID: %s", chatID)
}
