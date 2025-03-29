package dbmanager

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// Helper function to split SQL statements
func splitStatements(query string) []string {
	// Basic statement splitting - can be enhanced for more complex cases
	statements := strings.Split(query, ";")

	// Clean up statements
	var result []string
	for _, stmt := range statements {
		if stmt = strings.TrimSpace(stmt); stmt != "" {
			result = append(result, stmt)
		}
	}
	return result
}

// Update the processRows function signature to return results and error
func processRows(rows *sql.Rows, startTime time.Time) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}

	results := make([]map[string]interface{}, 0)
	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(columns))

	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(scanArgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val == nil {
				row[col] = nil
				continue
			}

			// Handle different types
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return results, nil
}

// Fix the extractTableName function to properly handle table names
func extractTableName(stmt string) string {
	// Preserve original case for the extraction
	originalStmt := strings.TrimSpace(stmt)
	upperStmt := strings.ToUpper(originalStmt)

	var tableName string

	// Handle different DROP TABLE variations
	if strings.HasPrefix(upperStmt, "DROP TABLE IF EXISTS") {
		tableName = strings.TrimSpace(originalStmt[len("DROP TABLE IF EXISTS"):])
	} else if strings.HasPrefix(upperStmt, "DROP TABLE") {
		tableName = strings.TrimSpace(originalStmt[len("DROP TABLE"):])
	} else {
		return ""
	}

	// Handle schema prefixes, quotes, and trailing characters
	tableName = strings.Split(tableName, " ")[0] // Get first word
	tableName = strings.Trim(tableName, "\"`;")  // Remove quotes and semicolons

	// Handle schema prefixes like "public."
	if strings.Contains(tableName, ".") {
		parts := strings.Split(tableName, ".")
		tableName = parts[len(parts)-1] // Get the last part after the dot
	}

	log.Printf("PostgresDriver -> extractTableName -> Extracted table name '%s' from statement: %s",
		tableName, originalStmt)

	return tableName
}

// Update the function signature to include indexes
func convertTablesToSchemaFormat(tables map[string]PostgresTable, indexes map[string][]PostgresIndex) map[string]TableSchema {
	result := make(map[string]TableSchema)
	for name, table := range tables {
		schema := TableSchema{
			Name:        name,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
			RowCount:    table.RowCount,
		}

		// Convert columns
		for colName, col := range table.Columns {
			schema.Columns[colName] = col.toColumnInfo()
		}

		// Convert indexes
		if tableIndexes, ok := indexes[name]; ok {
			for _, idx := range tableIndexes {
				schema.Indexes[idx.Name] = IndexInfo{
					Name:     idx.Name,
					Columns:  idx.Columns,
					IsUnique: idx.IsUnique,
				}
			}
		}

		// Convert foreign keys
		if table.ForeignKeys != nil {
			for fkName, fk := range table.ForeignKeys {
				schema.ForeignKeys[fkName] = ForeignKey{
					Name:       fk.Name,
					ColumnName: fk.Column,
					RefTable:   fk.RefTable,
					RefColumn:  fk.RefColumn,
					OnDelete:   fk.OnDelete,
					OnUpdate:   fk.OnUpdate,
				}
			}
		}

		result[name] = schema
	}
	return result
}
