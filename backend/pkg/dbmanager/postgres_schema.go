package dbmanager

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PostgresSchemaFetcher implements schema fetching for PostgreSQL
type PostgresSchemaFetcher struct {
	db DBExecutor
}

func (f *PostgresSchemaFetcher) FetchSchema(ctx context.Context) (*SchemaInfo, error) {
	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		UpdatedAt: time.Now(),
	}

	// Fetch tables
	tables, err := f.fetchTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
		tableSchema := TableSchema{
			Name:        table,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
		}

		// Fetch columns
		columns, err := f.fetchColumns(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.Columns = columns

		// Fetch indexes
		indexes, err := f.fetchIndexes(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.Indexes = indexes

		// Fetch foreign keys
		fks, err := f.fetchForeignKeys(ctx, table)
		if err != nil {
			return nil, err
		}
		tableSchema.ForeignKeys = fks

		// Calculate table checksum
		tableData, _ := json.Marshal(tableSchema)
		tableSchema.Checksum = fmt.Sprintf("%x", md5.Sum(tableData))

		schema.Tables[table] = tableSchema
	}

	// Calculate overall schema checksum
	schemaData, _ := json.Marshal(schema.Tables)
	schema.Checksum = fmt.Sprintf("%x", md5.Sum(schemaData))

	return schema, nil
}

func (f *PostgresSchemaFetcher) fetchTables(_ context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

func (f *PostgresSchemaFetcher) fetchColumns(_ context.Context, table string) (map[string]ColumnInfo, error) {
	columns := make(map[string]ColumnInfo)
	var columnList []struct {
		Name         string `db:"column_name"`
		Type         string `db:"data_type"`
		IsNullable   string `db:"is_nullable"`
		DefaultValue string `db:"column_default"`
		Comment      string `db:"column_comment"`
	}

	query := `
        SELECT 
            column_name,
            data_type,
            is_nullable,
            column_default,
            col_description((table_schema || '.' || table_name)::regclass::oid, ordinal_position) as column_comment
        FROM information_schema.columns c
        WHERE table_schema = 'public'
        AND table_name = $1
        ORDER BY ordinal_position;
    `
	err := f.db.Query(query, &columnList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch columns for table %s: %v", table, err)
	}

	for _, col := range columnList {
		columns[col.Name] = ColumnInfo{
			Name:         col.Name,
			Type:         col.Type,
			IsNullable:   col.IsNullable == "YES",
			DefaultValue: col.DefaultValue,
			Comment:      col.Comment,
		}
	}
	return columns, nil
}

func (f *PostgresSchemaFetcher) fetchIndexes(_ context.Context, table string) (map[string]IndexInfo, error) {
	indexes := make(map[string]IndexInfo)
	var indexList []struct {
		Name     string `db:"indexname"`
		Columns  string `db:"columns"`
		IsUnique bool   `db:"is_unique"`
	}

	query := `
        SELECT 
            i.relname as indexname,
            array_to_string(array_agg(a.attname), ',') as columns,
            idx.indisunique as is_unique
        FROM pg_index idx
        JOIN pg_class i ON i.oid = idx.indexrelid
        JOIN pg_class t ON t.oid = idx.indrelid
        JOIN pg_attribute a ON a.attrelid = t.oid
        WHERE t.relname = $1
        AND a.attnum = ANY(idx.indkey)
        GROUP BY i.relname, idx.indisunique;
    `
	err := f.db.Query(query, &indexList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch indexes for table %s: %v", table, err)
	}

	for _, idx := range indexList {
		indexes[idx.Name] = IndexInfo{
			Name:     idx.Name,
			Columns:  strings.Split(idx.Columns, ","),
			IsUnique: idx.IsUnique,
		}
	}
	return indexes, nil
}

func (f *PostgresSchemaFetcher) fetchForeignKeys(_ context.Context, table string) (map[string]ForeignKey, error) {
	fkeys := make(map[string]ForeignKey)
	var fkList []struct {
		Name       string `db:"constraint_name"`
		ColumnName string `db:"column_name"`
		RefTable   string `db:"ref_table"`
		RefColumn  string `db:"ref_column"`
		OnDelete   string `db:"on_delete"`
		OnUpdate   string `db:"on_update"`
	}

	query := `
        SELECT
            tc.constraint_name,
            kcu.column_name,
            ccu.table_name as ref_table,
            ccu.column_name as ref_column,
            rc.delete_rule as on_delete,
            rc.update_rule as on_update
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
            ON tc.constraint_name = kcu.constraint_name
        JOIN information_schema.constraint_column_usage ccu
            ON ccu.constraint_name = tc.constraint_name
        JOIN information_schema.referential_constraints rc
            ON tc.constraint_name = rc.constraint_name
        WHERE tc.table_name = $1
        AND tc.constraint_type = 'FOREIGN KEY';
    `
	err := f.db.Query(query, &fkList, table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch foreign keys for table %s: %v", table, err)
	}

	for _, fk := range fkList {
		fkeys[fk.Name] = ForeignKey{
			Name:       fk.Name,
			ColumnName: fk.ColumnName,
			RefTable:   fk.RefTable,
			RefColumn:  fk.RefColumn,
			OnDelete:   fk.OnDelete,
			OnUpdate:   fk.OnUpdate,
		}
	}
	return fkeys, nil
}

func (f *PostgresSchemaFetcher) FetchTableList(ctx context.Context) ([]string, error) {
	var tables []string
	query := `
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_type = 'BASE TABLE'
        ORDER BY table_name;
    `
	err := f.db.Query(query, &tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tables: %v", err)
	}
	return tables, nil
}

// Add GetTableChecksum method to PostgresSchemaFetcher
func (f *PostgresSchemaFetcher) GetTableChecksum(ctx context.Context, table string) (string, error) {
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

	err := f.db.Query(query, &tableDefinition, table)
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

	err = f.db.Query(indexQuery, &indexes, table)
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

	err = f.db.Query(fkQuery, &foreignKeys, table)
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

// Add FetchExampleRecords method to PostgresSchemaFetcher
func (f *PostgresSchemaFetcher) FetchExampleRecords(ctx context.Context, db DBExecutor, table string, limit int) ([]map[string]interface{}, error) {
	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
	}

	// Build a simple query to fetch example records
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d", table, limit)

	var records []map[string]interface{}
	err := db.QueryRows(query, &records)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch example records for table %s: %v", table, err)
	}

	// If no records found, return empty slice
	if len(records) == 0 {
		return []map[string]interface{}{}, nil
	}

	return records, nil
}
