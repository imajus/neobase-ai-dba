package dbmanager

import (
	"strings"
)

// MongoDBSimplifier implements SchemaSimplifier for MongoDB
type MongoDBSimplifier struct{}

// SimplifyDataType simplifies MongoDB data types for better readability
func (s *MongoDBSimplifier) SimplifyDataType(dbType string) string {
	// MongoDB types are already simplified during schema inference
	// but we can further simplify some types here
	switch dbType {
	case "objectId":
		return "ObjectID"
	case "number":
		return "Number"
	case "string":
		return "String"
	case "boolean":
		return "Boolean"
	case "date":
		return "Date"
	case "array":
		return "Array"
	case "object":
		return "Object"
	case "null":
		return "Null"
	default:
		return dbType
	}
}

// GetColumnConstraints returns constraints for a MongoDB column
func (s *MongoDBSimplifier) GetColumnConstraints(col ColumnInfo, table TableSchema) []string {
	constraints := []string{}

	// Check if the column is part of an index
	isIndexed := false
	isUnique := false
	isPrimary := false

	// Check if the column is part of any index
	for _, idx := range table.Indexes {
		for _, idxCol := range idx.Columns {
			if idxCol == col.Name {
				isIndexed = true
				if idx.IsUnique {
					isUnique = true
				}
				// In MongoDB, the _id field is always the primary key
				if col.Name == "_id" {
					isPrimary = true
				}
				break
			}
		}
		if isIndexed {
			break
		}
	}

	// Add constraints based on column properties
	if isPrimary {
		constraints = append(constraints, "PRIMARY KEY")
	}
	if isUnique && !isPrimary {
		constraints = append(constraints, "UNIQUE")
	}
	if isIndexed && !isUnique && !isPrimary {
		constraints = append(constraints, "INDEXED")
	}
	if !col.IsNullable {
		constraints = append(constraints, "NOT NULL")
	}

	// Special handling for MongoDB ObjectIDs
	if strings.ToLower(col.Type) == "objectid" {
		constraints = append(constraints, "MONGODB OBJECTID")
	}

	return constraints
}
