package dbmanager

import (
	"strings"
)

// ClickHouseSimplifier implements the SchemaSimplifier interface for ClickHouse
type ClickHouseSimplifier struct{}

// SimplifyDataType converts ClickHouse data types to simplified versions for LLM
func (s *ClickHouseSimplifier) SimplifyDataType(dbType string) string {
	// Convert to lowercase for consistent matching
	lowerType := strings.ToLower(dbType)

	// Remove Nullable wrapper if present
	if strings.HasPrefix(lowerType, "nullable(") && strings.HasSuffix(lowerType, ")") {
		lowerType = lowerType[9 : len(lowerType)-1]
	}

	// Integer types
	if strings.Contains(lowerType, "int") || strings.Contains(lowerType, "uint") {
		return "integer"
	}

	// Decimal/numeric types
	if strings.Contains(lowerType, "decimal") || strings.Contains(lowerType, "float") ||
		strings.Contains(lowerType, "double") {
		return "number"
	}

	// Date/time types
	if strings.Contains(lowerType, "date") || strings.Contains(lowerType, "time") {
		return "datetime"
	}

	// String types
	if strings.Contains(lowerType, "string") || strings.Contains(lowerType, "fixedstring") ||
		strings.Contains(lowerType, "enum") {
		return "string"
	}

	// Array types
	if strings.HasPrefix(lowerType, "array(") {
		return "array"
	}

	// Map types
	if strings.HasPrefix(lowerType, "map(") {
		return "map"
	}

	// Tuple types
	if strings.HasPrefix(lowerType, "tuple(") {
		return "tuple"
	}

	// Boolean type
	if lowerType == "bool" || lowerType == "boolean" {
		return "boolean"
	}

	// UUID type
	if lowerType == "uuid" {
		return "uuid"
	}

	// JSON type
	if strings.Contains(lowerType, "json") {
		return "json"
	}

	// Default to original type if no match
	return dbType
}

// GetColumnConstraints returns a list of constraints for a column
func (s *ClickHouseSimplifier) GetColumnConstraints(col ColumnInfo, table TableSchema) []string {
	var constraints []string

	// Check if column is nullable
	if !col.IsNullable {
		constraints = append(constraints, "NOT NULL")
	}

	// Check if column has a default value
	if col.DefaultValue != "" {
		constraints = append(constraints, col.DefaultValue)
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

	// ClickHouse doesn't have traditional foreign keys, unique constraints, etc.
	// But we can add engine-specific information if needed
	if table.Comment != "" && strings.Contains(strings.ToLower(table.Comment), "engine=") {
		engineInfo := extractEngineInfo(table.Comment)
		if engineInfo != "" {
			constraints = append(constraints, "ENGINE: "+engineInfo)
		}
	}

	// Add partition key information if available
	if partitionKey := extractPartitionKey(table.Comment); partitionKey != "" {
		constraints = append(constraints, "PARTITION KEY: "+partitionKey)
	}

	// Add order by key information if available
	if orderByKey := extractOrderByKey(table.Comment); orderByKey != "" {
		constraints = append(constraints, "ORDER BY: "+orderByKey)
	}

	return constraints
}

// extractEngineInfo extracts the engine information from the table comment
func extractEngineInfo(comment string) string {
	lowerComment := strings.ToLower(comment)
	if idx := strings.Index(lowerComment, "engine="); idx != -1 {
		// Extract from "engine=" to the next space or end of string
		engineStart := idx + 7 // length of "engine="
		engineEnd := len(comment)

		if spaceIdx := strings.Index(comment[engineStart:], " "); spaceIdx != -1 {
			engineEnd = engineStart + spaceIdx
		}

		return comment[engineStart:engineEnd]
	}
	return ""
}

// extractPartitionKey extracts the partition key from the table comment
func extractPartitionKey(comment string) string {
	lowerComment := strings.ToLower(comment)
	if idx := strings.Index(lowerComment, "partition by"); idx != -1 {
		// Extract from "partition by" to the next keyword or end of string
		partitionStart := idx + 12 // length of "partition by"
		partitionEnd := len(comment)

		for _, keyword := range []string{"order by", "primary key", "sample by", "settings"} {
			if keywordIdx := strings.Index(strings.ToLower(comment[partitionStart:]), keyword); keywordIdx != -1 {
				if partitionStart+keywordIdx < partitionEnd {
					partitionEnd = partitionStart + keywordIdx
				}
			}
		}

		return strings.TrimSpace(comment[partitionStart:partitionEnd])
	}
	return ""
}

// extractOrderByKey extracts the order by key from the table comment
func extractOrderByKey(comment string) string {
	lowerComment := strings.ToLower(comment)
	if idx := strings.Index(lowerComment, "order by"); idx != -1 {
		// Extract from "order by" to the next keyword or end of string
		orderStart := idx + 8 // length of "order by"
		orderEnd := len(comment)

		for _, keyword := range []string{"partition by", "primary key", "sample by", "settings"} {
			if keywordIdx := strings.Index(strings.ToLower(comment[orderStart:]), keyword); keywordIdx != -1 {
				if orderStart+keywordIdx < orderEnd {
					orderEnd = orderStart + keywordIdx
				}
			}
		}

		return strings.TrimSpace(comment[orderStart:orderEnd])
	}
	return ""
}
