package dbmanager

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"neobase-ai/pkg/redis"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// Add these constants
const (
	schemaKeyPrefix = "schema:"
	schemaTTL       = 24 * time.Hour // Keep schemas for 24 hours
)

// SchemaInfo represents database schema information
type SchemaInfo struct {
	Tables    map[string]TableSchema `json:"tables"`
	UpdatedAt time.Time              `json:"updated_at"`
	Checksum  string                 `json:"checksum"`
}

type TableSchema struct {
	Name        string                `json:"name"`
	Columns     map[string]ColumnInfo `json:"columns"`
	Indexes     map[string]IndexInfo  `json:"indexes"`
	ForeignKeys map[string]ForeignKey `json:"foreign_keys"`
	Comment     string                `json:"comment,omitempty"`
	Checksum    string                `json:"checksum"` // For individual table changes
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

// Add new types for database-agnostic schema handling
type SchemaFetcher interface {
	FetchSchema(ctx context.Context) (*SchemaInfo, error)
	FetchTableList(ctx context.Context) ([]string, error)
	GetTableChecksum(ctx context.Context, table string) (string, error)
}

// SchemaManager handles schema tracking and diffing
type SchemaManager struct {
	redisRepo   redis.IRedisRepositories
	mu          sync.RWMutex
	schemaCache map[string]*SchemaInfo
	encryption  *SchemaEncryption
	fetcherMap  map[string]func(DBExecutor) SchemaFetcher // Maps DB type to fetcher constructor
}

func NewSchemaManager(redisRepo redis.IRedisRepositories, encryptionKey string) (*SchemaManager, error) {
	encryption, err := NewSchemaEncryption(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema encryption: %v", err)
	}

	sm := &SchemaManager{
		redisRepo:   redisRepo,
		schemaCache: make(map[string]*SchemaInfo),
		encryption:  encryption,
		fetcherMap:  make(map[string]func(DBExecutor) SchemaFetcher),
	}

	// Register default fetchers
	sm.RegisterFetcher("postgresql", func(db DBExecutor) SchemaFetcher {
		return &PostgresSchemaFetcher{db: db}
	})
	// Add more database types as needed
	// sm.RegisterFetcher("mysql", func(db DBExecutor) SchemaFetcher {
	//     return &MySQLSchemaFetcher{db: db}
	// })

	return sm, nil
}

// RegisterFetcher registers a new schema fetcher for a database type
func (sm *SchemaManager) RegisterFetcher(dbType string, constructor func(DBExecutor) SchemaFetcher) {
	sm.fetcherMap[dbType] = constructor
}

// getFetcher returns appropriate schema fetcher for the database type
func (sm *SchemaManager) getFetcher(dbType string, db DBExecutor) (SchemaFetcher, error) {
	constructor, exists := sm.fetcherMap[dbType]
	if !exists {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	return constructor(db), nil
}

// Update schema fetching methods to use appropriate fetcher
func (sm *SchemaManager) fetchSchema(ctx context.Context, db DBExecutor, dbType string) (*SchemaInfo, error) {
	fetcher, err := sm.getFetcher(dbType, db)
	if err != nil {
		return nil, err
	}
	return fetcher.FetchSchema(ctx)
}

// Update GetSchema to include dbType
func (sm *SchemaManager) GetSchema(ctx context.Context, chatID string, db DBExecutor, dbType string) (*SchemaInfo, error) {
	sm.mu.RLock()
	cachedSchema, exists := sm.schemaCache[chatID]
	sm.mu.RUnlock()

	if exists {
		return cachedSchema, nil
	}

	schema, err := sm.fetchSchema(ctx, db, dbType)
	if err != nil {
		return nil, err
	}

	sm.mu.Lock()
	sm.schemaCache[chatID] = schema
	sm.mu.Unlock()

	return schema, sm.storeSchema(ctx, chatID, schema)
}

// Update CheckSchemaChanges to include dbType
func (sm *SchemaManager) CheckSchemaChanges(ctx context.Context, chatID string, db DBExecutor, dbType string) (*SchemaDiff, error) {
	currentSchema, err := sm.fetchSchema(ctx, db, dbType)
	if err != nil {
		return nil, err
	}

	storage, err := sm.getStoredSchema(ctx, chatID)
	if err != nil {
		// If no previous schema, treat current as initial
		if err := sm.storeSchema(ctx, chatID, currentSchema); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Compare schemas
	if currentSchema.Checksum == storage.FullSchema.Checksum {
		return nil, nil
	}

	// Generate diff
	diff := sm.generateDiff(storage.FullSchema, currentSchema)
	if diff == nil {
		return nil, nil
	}

	// Update cache and Redis
	sm.mu.Lock()
	sm.schemaCache[chatID] = currentSchema
	sm.mu.Unlock()

	if err := sm.storeSchema(ctx, chatID, currentSchema); err != nil {
		return nil, err
	}

	return diff, nil
}

// generateDiff creates a detailed diff between two schemas
func (sm *SchemaManager) generateDiff(old, new *SchemaInfo) *SchemaDiff {
	diff := &SchemaDiff{
		ModifiedTables: make(map[string]TableDiff),
		UpdatedAt:      time.Now(),
	}

	// Check for added/removed tables
	for tableName := range new.Tables {
		if _, exists := old.Tables[tableName]; !exists {
			diff.AddedTables = append(diff.AddedTables, tableName)
		}
	}

	for tableName := range old.Tables {
		if _, exists := new.Tables[tableName]; !exists {
			diff.RemovedTables = append(diff.RemovedTables, tableName)
		}
	}

	// Check for modified tables
	for tableName, newTable := range new.Tables {
		oldTable, exists := old.Tables[tableName]
		if !exists {
			continue
		}

		if newTable.Checksum != oldTable.Checksum {
			tableDiff := sm.generateTableDiff(&oldTable, &newTable)
			if tableDiff != nil {
				diff.ModifiedTables[tableName] = *tableDiff
			}
		}
	}

	if len(diff.AddedTables) == 0 && len(diff.RemovedTables) == 0 && len(diff.ModifiedTables) == 0 {
		return nil
	}

	return diff
}

// generateTableDiff creates a detailed diff for a single table
func (sm *SchemaManager) generateTableDiff(old, new *TableSchema) *TableDiff {
	diff := &TableDiff{}

	// Check columns
	for colName := range new.Columns {
		if _, exists := old.Columns[colName]; !exists {
			diff.AddedColumns = append(diff.AddedColumns, colName)
		}
	}

	for colName := range old.Columns {
		if _, exists := new.Columns[colName]; !exists {
			diff.RemovedColumns = append(diff.RemovedColumns, colName)
		}
	}

	// Check modified columns
	for colName, newCol := range new.Columns {
		oldCol, exists := old.Columns[colName]
		if !exists {
			continue
		}

		if !columnsEqual(oldCol, newCol) {
			diff.ModifiedColumns = append(diff.ModifiedColumns, colName)
		}
	}

	// Similar checks for indexes and foreign keys...

	if len(diff.AddedColumns) == 0 && len(diff.RemovedColumns) == 0 &&
		len(diff.ModifiedColumns) == 0 && len(diff.AddedIndexes) == 0 &&
		len(diff.RemovedIndexes) == 0 && len(diff.AddedFKs) == 0 &&
		len(diff.RemovedFKs) == 0 {
		return nil
	}

	return diff
}

// Add these methods to SchemaManager
func (sm *SchemaManager) storeSchema(ctx context.Context, chatID string, schema *SchemaInfo) error {
	// Create storage object
	storage := &SchemaStorage{
		FullSchema:     schema,
		LLMSchema:      sm.createLLMSchema(schema, "postgresql"),
		TableChecksums: make(map[string]string),
		UpdatedAt:      time.Now(),
	}

	// Calculate table checksums
	for tableName, table := range schema.Tables {
		storage.TableChecksums[tableName] = table.Checksum
	}

	// Compress the schema
	compressed, err := sm.compressSchema(storage)
	if err != nil {
		return err
	}

	// Encrypt the compressed data
	encrypted, err := sm.encryption.Encrypt(compressed)
	if err != nil {
		return fmt.Errorf("failed to encrypt schema: %v", err)
	}

	// Convert encrypted string to bytes
	encryptedBytes := []byte(encrypted)

	key := fmt.Sprintf("%s%s", schemaKeyPrefix, chatID)
	return sm.redisRepo.Set(key, encryptedBytes, schemaTTL, ctx)
}

// Compress schema for storage
func (sm *SchemaManager) compressSchema(storage *SchemaStorage) ([]byte, error) {
	// Marshal to JSON first
	data, err := json.Marshal(storage)
	if err != nil {
		return nil, err
	}

	// Use zlib compression
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Create LLM-friendly schema
func (sm *SchemaManager) createLLMSchema(schema *SchemaInfo, dbType string) *LLMSchemaInfo {
	var simplifier SchemaSimplifier
	switch dbType {
	case "postgresql":
		simplifier = &PostgresSimplifier{}
	case "mysql":
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
		}

		// Convert columns
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

// QuickSchemaCheck performs a fast check for schema changes
func (sm *SchemaManager) QuickSchemaCheck(ctx context.Context, chatID string, db DBExecutor) (bool, error) {
	storage, err := sm.getStoredSchema(ctx, chatID)
	if err != nil {
		return true, nil // If no cache or error, assume changed
	}

	// Quick check using table checksums
	currentChecksums, err := sm.getTableChecksums(ctx, db)
	if err != nil {
		return true, nil
	}

	return !reflect.DeepEqual(storage.TableChecksums, currentChecksums), nil
}

// Get current table checksums without fetching full schema
func (sm *SchemaManager) getTableChecksums(ctx context.Context, db DBExecutor) (map[string]string, error) {
	checksums := make(map[string]string)
	tables, err := sm.fetchTableList(ctx, db)
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
		checksum, err := sm.calculateTableChecksum(ctx, db, table)
		if err != nil {
			return nil, err
		}
		checksums[table] = checksum
	}

	return checksums, nil
}

// Update schema retrieval methods
func (sm *SchemaManager) getStoredSchema(ctx context.Context, chatID string) (*SchemaStorage, error) {
	key := fmt.Sprintf("%s%s", schemaKeyPrefix, chatID)
	encryptedData, err := sm.redisRepo.Get(key, ctx)
	if err != nil {
		return nil, err
	}

	// Decrypt the data
	decrypted, err := sm.encryption.Decrypt(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt schema: %v", err)
	}

	// Decompress the data
	var storage SchemaStorage
	if err := sm.decompressSchema(decrypted, &storage); err != nil {
		return nil, err
	}

	return &storage, nil
}

// Add helper functions
func columnsEqual(a, b ColumnInfo) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		a.IsNullable == b.IsNullable &&
		a.DefaultValue == b.DefaultValue &&
		a.Comment == b.Comment
}

// Add decompressSchema method
func (sm *SchemaManager) decompressSchema(data []byte, storage *SchemaStorage) error {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create zlib reader: %v", err)
	}
	defer r.Close()

	// Read all decompressed data
	decompressed, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to decompress data: %v", err)
	}

	// Unmarshal into storage
	return json.Unmarshal(decompressed, storage)
}

// Add fetchTableList method
func (sm *SchemaManager) fetchTableList(ctx context.Context, db DBExecutor) ([]string, error) {
	schema, err := db.GetSchema(ctx)
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0, len(schema.Tables))
	for tableName := range schema.Tables {
		tables = append(tables, tableName)
	}
	return tables, nil
}

// Add calculateTableChecksum method
func (sm *SchemaManager) calculateTableChecksum(ctx context.Context, db DBExecutor, table string) (string, error) {
	return db.GetTableChecksum(ctx, table)
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

// Add a quick check method
func (sm *SchemaManager) HasSchemaChanged(ctx context.Context, chatID string, db DBExecutor) (bool, error) {
	// 1. Try cache first
	sm.mu.RLock()
	cachedSchema, exists := sm.schemaCache[chatID]
	sm.mu.RUnlock()

	if exists {
		// Quick checksum comparison with database
		currentChecksums, err := sm.getTableChecksums(ctx, db)
		if err != nil {
			return true, nil // Assume changed on error
		}

		// Compare only checksums
		for tableName, checksum := range currentChecksums {
			if cachedSchema.Tables[tableName].Checksum != checksum {
				return true, nil
			}
		}
		return false, nil
	}

	// 2. If not in cache, check Redis
	storage, err := sm.getStoredSchema(ctx, chatID)
	if err != nil {
		return true, nil // Assume changed if no stored schema
	}

	// 3. Quick checksum comparison
	currentChecksums, err := sm.getTableChecksums(ctx, db)
	if err != nil {
		return true, nil
	}

	return !reflect.DeepEqual(storage.TableChecksums, currentChecksums), nil
}
