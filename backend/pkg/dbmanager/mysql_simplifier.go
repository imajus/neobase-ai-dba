package dbmanager

import (
	"strings"
)

// MySQLSimplifier implements the SchemaSimplifier interface for MySQL
type MySQLSimplifier struct{}

// SimplifyDataType converts MySQL data types to simplified versions for LLM
func (s *MySQLSimplifier) SimplifyDataType(dbType string) string {
	// Convert to lowercase for consistent matching
	lowerType := strings.ToLower(dbType)

	// Integer types
	if strings.Contains(lowerType, "int") || strings.Contains(lowerType, "bit") {
		return "integer"
	}

	// Decimal/numeric types
	if strings.Contains(lowerType, "decimal") || strings.Contains(lowerType, "numeric") ||
		strings.Contains(lowerType, "float") || strings.Contains(lowerType, "double") {
		return "number"
	}

	// Date/time types
	if strings.Contains(lowerType, "date") || strings.Contains(lowerType, "time") ||
		strings.Contains(lowerType, "year") {
		return "datetime"
	}

	// Text types
	if strings.Contains(lowerType, "char") || strings.Contains(lowerType, "text") ||
		strings.Contains(lowerType, "enum") || strings.Contains(lowerType, "set") {
		return "string"
	}

	// Binary types
	if strings.Contains(lowerType, "blob") || strings.Contains(lowerType, "binary") {
		return "binary"
	}

	// JSON type
	if strings.Contains(lowerType, "json") {
		return "json"
	}

	// Default to original type if no match
	return dbType
}

// GetColumnConstraints returns a list of constraints for a column
func (s *MySQLSimplifier) GetColumnConstraints(col ColumnInfo, table TableSchema) []string {
	var constraints []string

	// Check if column is nullable
	if !col.IsNullable {
		constraints = append(constraints, "NOT NULL")
	}

	// Check if column has a default value
	if col.DefaultValue != "" {
		constraints = append(constraints, "DEFAULT "+col.DefaultValue)
	}

	// Check if column is part of primary key
	for _, constraint := range table.Constraints {
		if constraint.Type == "PRIMARY KEY" {
			for _, colName := range constraint.Columns {
				if colName == col.Name {
					constraints = append(constraints, "PRIMARY KEY")
					break
				}
			}
		}
	}

	// Check if column has unique constraint
	isUnique := false
	for _, index := range table.Indexes {
		if index.IsUnique && len(index.Columns) == 1 && index.Columns[0] == col.Name {
			isUnique = true
			break
		}
	}
	for _, constraint := range table.Constraints {
		if constraint.Type == "UNIQUE" {
			for _, colName := range constraint.Columns {
				if colName == col.Name && len(constraint.Columns) == 1 {
					isUnique = true
					break
				}
			}
		}
	}
	if isUnique {
		constraints = append(constraints, "UNIQUE")
	}

	// Check if column is a foreign key
	for _, fk := range table.ForeignKeys {
		if fk.ColumnName == col.Name {
			constraints = append(constraints, "FOREIGN KEY REFERENCES "+fk.RefTable+"("+fk.RefColumn+")")
			break
		}
	}

	// Check if column is auto-increment (look for "auto_increment" in default value or type)
	if strings.Contains(strings.ToLower(col.DefaultValue), "auto_increment") ||
		strings.Contains(strings.ToLower(col.Type), "auto_increment") {
		constraints = append(constraints, "AUTO_INCREMENT")
	}

	return constraints
}
