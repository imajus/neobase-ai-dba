package dbmanager

// Add these constants
const (
	QueryTypeUnknown = "UNKNOWN"
	QueryTypeDDL     = "DDL"
	QueryTypeDML     = "DML"
	QueryTypeSelect  = "SELECT"
)

// Add these types for PostgreSQL schema tracking
type PostgresSchema struct {
	Tables      map[string]PostgresTable
	Indexes     map[string][]PostgresIndex
	Views       map[string]PostgresView
	Sequences   map[string]PostgresSequence     // For auto-increment/serial
	Constraints map[string][]PostgresConstraint // Table constraints (CHECK, UNIQUE, etc.)
	Enums       map[string]PostgresEnum         // Custom enum types
}

type PostgresTable struct {
	Name        string
	Columns     map[string]PostgresColumn
	Indexes     map[string]PostgresIndex
	PrimaryKey  []string
	ForeignKeys map[string]PostgresForeignKey
	RowCount    int64
}

type PostgresColumn struct {
	Name         string
	Type         string
	IsNullable   bool
	DefaultValue string
	Comment      string
}

type PostgresIndex struct {
	Name      string
	Columns   []string
	IsUnique  bool
	TableName string
}

type PostgresView struct {
	Name       string
	Definition string
}

type PostgresForeignKey struct {
	Name      string
	Column    string
	RefTable  string
	RefColumn string
	OnDelete  string
	OnUpdate  string
}

// Add new types for additional schema elements
type PostgresSequence struct {
	Name       string
	StartValue int64
	Increment  int64
	MinValue   int64
	MaxValue   int64
	CacheSize  int64
	IsCycled   bool
}

type PostgresConstraint struct {
	Name       string
	Type       string // CHECK, UNIQUE, etc.
	TableName  string
	Definition string
	Columns    []string
}

type PostgresEnum struct {
	Name   string
	Values []string
	Schema string
}

// Add a conversion method for PostgresColumn
func (pc PostgresColumn) toColumnInfo() ColumnInfo {
	return ColumnInfo{
		Name:         pc.Name,
		Type:         pc.Type,
		IsNullable:   pc.IsNullable,
		DefaultValue: pc.DefaultValue,
		Comment:      pc.Comment,
	}
}
