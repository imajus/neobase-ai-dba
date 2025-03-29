package dbmanager

import "strings"

// ClickHouse schema structures
type ClickHouseSchema struct {
	Tables       map[string]ClickHouseTable
	Views        map[string]ClickHouseView
	Dictionaries map[string]ClickHouseDictionary
}

type ClickHouseTable struct {
	Name         string
	Columns      map[string]ClickHouseColumn
	Engine       string
	PartitionKey string
	OrderBy      string
	PrimaryKey   []string
	RowCount     int64
}

type ClickHouseColumn struct {
	Name         string
	Type         string
	IsNullable   bool
	DefaultValue string
	Comment      string
}

type ClickHouseView struct {
	Name       string
	Definition string
}

type ClickHouseDictionary struct {
	Name       string
	Definition string
}

// splitClickHouseStatements splits a ClickHouse query string into individual statements
func splitClickHouseStatements(query string) []string {
	// Split by semicolons, but handle cases where semicolons are within quotes
	var statements []string
	var currentStmt strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, char := range query {
		switch char {
		case '\'', '"', '`':
			if inQuote && char == quoteChar {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quoteChar = char
			}
			currentStmt.WriteRune(char)
		case ';':
			if inQuote {
				currentStmt.WriteRune(char)
			} else {
				statements = append(statements, currentStmt.String())
				currentStmt.Reset()
			}
		default:
			currentStmt.WriteRune(char)
		}
	}

	// Add the last statement if there's anything left
	if currentStmt.Len() > 0 {
		statements = append(statements, currentStmt.String())
	}

	return statements
}
