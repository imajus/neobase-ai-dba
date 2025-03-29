package dbmanager

import "strings"

// MySQL schema structures
type MySQLSchema struct {
	Tables      map[string]MySQLTable
	Indexes     map[string][]MySQLIndex
	Views       map[string]MySQLView
	Constraints map[string][]MySQLConstraint
}

type MySQLTable struct {
	Name        string
	Columns     map[string]MySQLColumn
	Indexes     map[string]MySQLIndex
	PrimaryKey  []string
	ForeignKeys map[string]MySQLForeignKey
	RowCount    int64
}

type MySQLColumn struct {
	Name         string
	Type         string
	IsNullable   bool
	DefaultValue string
	Comment      string
}

type MySQLIndex struct {
	Name      string
	Columns   []string
	IsUnique  bool
	TableName string
}

type MySQLView struct {
	Name       string
	Definition string
}

type MySQLForeignKey struct {
	Name      string
	Column    string
	RefTable  string
	RefColumn string
	OnDelete  string
	OnUpdate  string
}

type MySQLConstraint struct {
	Name       string
	Type       string // PRIMARY KEY, UNIQUE, CHECK, etc.
	TableName  string
	Definition string
	Columns    []string
}

// splitMySQLStatements splits a MySQL query string into individual statements
func splitMySQLStatements(query string) []string {
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
