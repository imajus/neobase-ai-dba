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
	// First try to get from cache
	sm.mu.RLock()
	cachedSchema, exists := sm.schemaCache[chatID]
	sm.mu.RUnlock()

	if exists {
		log.Printf("SchemaManager -> GetSchema -> Using cached schema for chatID: %s", chatID)
		return cachedSchema, nil
	}

	// If not in cache, fetch using appropriate fetcher
	schema, err := sm.fetchSchema(ctx, db, dbType)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schema: %v", err)
	}

	// Cache the schema
	sm.mu.Lock()
	sm.schemaCache[chatID] = schema
	sm.mu.Unlock()

	return schema, nil
}

// Update CheckSchemaChanges to include dbType
func (sm *SchemaManager) CheckSchemaChanges(ctx context.Context, chatID string, db DBExecutor, dbType string) (*SchemaDiff, bool, error) {
	_, exists := sm.dbManager.drivers[dbType]
	if !exists {
		return nil, false, fmt.Errorf("no driver found for type: %s", dbType)
	}

	// Get current schema using driver
	currentSchema, err := sm.GetSchema(ctx, chatID, db, dbType)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get current schema: %v", err)
	}

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
			FullSchema:  currentSchema, // Add this field to SchemaDiff struct
			UpdatedAt:   time.Now(),
			IsFirstTime: true,
		}, true, nil
	}

	// Normal comparison for subsequent changes
	diff := sm.compareSchemas(storedSchema.FullSchema, currentSchema)
	if diff == nil {
		return nil, false, nil
	}

	// Store updated schema
	if err := sm.storeSchema(ctx, chatID, currentSchema, db, dbType); err != nil {
		return nil, true, err
	}

	return diff, true, nil
}

// Update compareSchemas to ignore row counts
func (sm *SchemaManager) compareSchemas(old, new *SchemaInfo) *SchemaDiff {
	if old == nil {
		return &SchemaDiff{
			IsFirstTime: true,
			FullSchema:  new,
			UpdatedAt:   time.Now(),
		}
	}

	diff := &SchemaDiff{
		AddedTables:    make([]string, 0),
		RemovedTables:  make([]string, 0),
		ModifiedTables: make(map[string]TableDiff),
		UpdatedAt:      time.Now(),
	}

	// Compare tables
	for tableName, newTable := range new.Tables {
		if oldTable, exists := old.Tables[tableName]; !exists {
			diff.AddedTables = append(diff.AddedTables, tableName)
		} else {
			// Compare table structure (ignoring row count)
			tableDiff := compareTableSchemas(oldTable, newTable)
			if !tableDiff.isEmpty() {
				diff.ModifiedTables[tableName] = tableDiff
			}
		}
	}

	// Check for removed tables
	for tableName := range old.Tables {
		if _, exists := new.Tables[tableName]; !exists {
			diff.RemovedTables = append(diff.RemovedTables, tableName)
		}
	}

	return diff
}

// Update compareTableSchemas to ignore row count
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
			diff.AddedColumns = append(diff.AddedColumns, colName)
		} else if !compareColumns(oldCol, newCol) {
			diff.ModifiedColumns = append(diff.ModifiedColumns, colName)
		}
	}

	// Check for removed columns
	for colName := range old.Columns {
		if _, exists := new.Columns[colName]; !exists {
			diff.RemovedColumns = append(diff.RemovedColumns, colName)
		}
	}

	// Compare indexes (unchanged)
	for idxName := range new.Indexes {
		if _, exists := old.Indexes[idxName]; !exists {
			diff.AddedIndexes = append(diff.AddedIndexes, idxName)
		}
	}

	for idxName := range old.Indexes {
		if _, exists := new.Indexes[idxName]; !exists {
			diff.RemovedIndexes = append(diff.RemovedIndexes, idxName)
		}
	}

	// Compare foreign keys (unchanged)
	for fkName := range new.ForeignKeys {
		if _, exists := old.ForeignKeys[fkName]; !exists {
			diff.AddedFKs = append(diff.AddedFKs, fkName)
		}
	}

	for fkName := range old.ForeignKeys {
		if _, exists := new.ForeignKeys[fkName]; !exists {
			diff.RemovedFKs = append(diff.RemovedFKs, fkName)
		}
	}

	return diff
}

// Helper function to compare columns
func compareColumns(old, new ColumnInfo) bool {
	return old.Name == new.Name &&
		old.Type == new.Type &&
		old.IsNullable == new.IsNullable &&
		old.DefaultValue == new.DefaultValue &&
		old.Comment == new.Comment
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
	switch dbType {
	case constants.DatabaseTypePostgreSQL:
		checksums := make(map[string]string)

		// Get schema directly from the database
		schema, err := db.GetSchema(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get schema: %v", err)
		}

		// Calculate checksums for each table
		for tableName, table := range schema.Tables {
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

// Update calculateTableChecksum to use driver directly
func (sm *SchemaManager) calculateTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	return db.GetTableChecksum(ctx, table)
}

// Update QuickSchemaCheck to use fetchTableList and calculateTableChecksum
func (sm *SchemaManager) QuickSchemaCheck(ctx context.Context, chatID string, db DBExecutor) (bool, error) {
	storedSchema, err := sm.getStoredSchema(ctx, chatID)
	if err != nil {
		return true, fmt.Errorf("failed to get stored schema: %v", err)
	}

	currentTables, err := sm.fetchTableList(ctx, db)
	if err != nil {
		return false, fmt.Errorf("failed to fetch current tables: %v", err)
	}

	// Quick check: compare table counts and names
	storedTables := make([]string, 0, len(storedSchema.FullSchema.Tables))
	for tableName := range storedSchema.FullSchema.Tables {
		storedTables = append(storedTables, tableName)
	}
	sort.Strings(storedTables)

	if !reflect.DeepEqual(currentTables, storedTables) {
		return true, nil
	}

	// Compare table checksums
	for _, tableName := range currentTables {
		currentChecksum, err := sm.calculateTableChecksum(ctx, db, tableName)
		if err != nil {
			log.Printf("QuickSchemaCheck -> Error calculating checksum for table %s: %v", tableName, err)
			continue
		}

		storedChecksum, exists := storedSchema.TableChecksums[tableName]
		if !exists || storedChecksum != currentChecksum {
			return true, nil
		}
	}

	return false, nil
}

// Update schema retrieval methods
func (sm *SchemaManager) getStoredSchema(ctx context.Context, chatID string) (*SchemaStorage, error) {
	return sm.storageService.Retrieve(ctx, chatID)
}

// Add helper functions
func columnsEqual(a, b ColumnInfo) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		a.IsNullable == b.IsNullable &&
		a.DefaultValue == b.DefaultValue &&
		a.Comment == b.Comment
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
	}

	return result.String()
}

// Update HasSchemaChanged to properly compare checksums
func (sm *SchemaManager) HasSchemaChanged(ctx context.Context, chatID string, db DBExecutor) (bool, error) {
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

// Update TriggerSchemaCheck to handle different trigger types
func (sm *SchemaManager) TriggerSchemaCheck(chatID string, triggerType TriggerType) (bool, error) {
	log.Printf("SchemaManager -> TriggerSchemaCheck -> Starting for chatID: %s, triggerType: %s", chatID, triggerType)

	// Get current connection
	db, err := sm.dbManager.GetConnection(chatID)
	if err != nil {
		return false, fmt.Errorf("failed to get connection: %v", err)
	}

	// Get connection config
	conn, exists := sm.dbManager.connections[chatID]
	if !exists {
		return false, fmt.Errorf("connection not found for chatID: %s", chatID)
	}

	if triggerType == TriggerTypeManual {
		// For manual triggers (DDL), directly fetch and store new schema
		log.Printf("SchemaManager -> TriggerSchemaCheck -> Manual trigger, fetching new schema")
		schema, err := db.GetSchema(context.Background())
		if err != nil {
			return false, fmt.Errorf("failed to get current schema: %v", err)
		}

		// Store the fresh schema immediately
		if err := sm.storeSchema(context.Background(), chatID, schema, db, conn.Config.Type); err != nil {
			return false, fmt.Errorf("failed to store schema: %v", err)
		}

		return true, nil
	}

	// For auto triggers, check for changes first
	hasChanged, err := sm.QuickSchemaCheck(context.Background(), chatID, db)
	log.Printf("SchemaManager -> TriggerSchemaCheck -> hasChanged: %t", hasChanged)

	if err != nil {
		log.Printf("SchemaManager -> TriggerSchemaCheck -> Error in quick check: %v", err)
		return false, fmt.Errorf("error in quick check: %v", err)
	} else if !hasChanged {
		log.Printf("SchemaManager -> TriggerSchemaCheck -> No schema changes detected in quick check")
		return false, nil
	}

	// If changes detected, get fresh schema
	schema, err := db.GetSchema(context.Background())
	if err != nil {
		return false, fmt.Errorf("failed to get current schema: %v", err)
	}

	// Store the fresh schema and get detailed changes
	if err := sm.storeSchema(context.Background(), chatID, schema, db, conn.Config.Type); err != nil {
		return false, fmt.Errorf("failed to store schema: %v", err)
	}

	// Get and log detailed changes
	diff, hasChanged, err := sm.CheckSchemaChanges(context.Background(), chatID, db, conn.Config.Type)
	if err != nil {
		log.Printf("SchemaManager -> TriggerSchemaCheck -> Error checking changes: %v", err)
		return false, fmt.Errorf("error checking changes: %v", err)
	}

	if diff != nil && !diff.IsFirstTime {
		log.Printf("SchemaManager -> TriggerSchemaCheck -> Schema changes detected: %+v", diff)
	}

	return hasChanged, nil
}

// Helper function to get the latest schema
func (sm *SchemaManager) GetLatestSchema(chatID string) (*SchemaInfo, error) {
	// Get current connection
	db, err := sm.dbManager.GetConnection(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %v", err)
	}

	// Get connection config
	conn, exists := sm.dbManager.connections[chatID]
	if !exists {
		return nil, fmt.Errorf("connection not found for chatID: %s", chatID)
	}

	// For manual triggers (DDL), directly fetch and store new schema
	log.Printf("SchemaManager -> RefreshSchema -> Manual trigger, fetching new schema")
	schema, err := db.GetSchema(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get current schema: %v", err)
	}

	// Store the fresh schema immediately
	if err := sm.storeSchema(context.Background(), chatID, schema, db, conn.Config.Type); err != nil {
		return nil, fmt.Errorf("failed to store schema: %v", err)
	}

	return schema, nil
}

// Add method to TableDiff to check if it's empty
func (td TableDiff) isEmpty() bool {
	return len(td.AddedColumns) == 0 &&
		len(td.RemovedColumns) == 0 &&
		len(td.ModifiedColumns) == 0 &&
		len(td.AddedIndexes) == 0 &&
		len(td.RemovedIndexes) == 0 &&
		len(td.AddedFKs) == 0 &&
		len(td.RemovedFKs) == 0
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
	case constants.DatabaseTypePostgreSQL:
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

	// Register MySQL fetcher when implemented
	// sm.RegisterFetcher("mysql", func(db DBExecutor) SchemaFetcher {
	//     return &MySQLDriver{}
	// })
}
