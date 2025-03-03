package services

import (
	"context"
	"fmt"
	"neobase-ai/pkg/dbmanager"
	"strings"
)

// VisualizationType represents the type of visualization
type VisualizationType string

const (
	BarChart        VisualizationType = "bar_chart"
	LineChart       VisualizationType = "line_chart"
	PieChart        VisualizationType = "pie_chart"
	Table           VisualizationType = "table"
	TimeSeriesChart VisualizationType = "time_series"
)

// VisualizationSuggestion represents a suggested visualization for a database table
type VisualizationSuggestion struct {
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	VisualizationType VisualizationType `json:"visualization_type"`
	Query             string            `json:"query"`
	TableName         string            `json:"table_name"`
}

// VisualizationData represents the data for a visualization
type VisualizationData struct {
	Title             string                 `json:"title"`
	Description       string                 `json:"description"`
	VisualizationType VisualizationType      `json:"visualization_type"`
	Query             string                 `json:"query"`
	TableName         string                 `json:"table_name"`
	Data              map[string]interface{} `json:"data"`
	ExecutionTime     int                    `json:"execution_time"`
}

// VisualizationService defines the interface for visualization operations
type VisualizationService interface {
	GetTableSuggestions(ctx context.Context, chatID, tableName string) ([]VisualizationSuggestion, error)
	GetAllSuggestions(ctx context.Context, chatID string) (map[string][]VisualizationSuggestion, error)
	ExecuteVisualization(ctx context.Context, chatID, streamID string, suggestion VisualizationSuggestion) (*VisualizationData, error)
}

// visualizationService implements VisualizationService
type visualizationService struct {
	dbManager *dbmanager.Manager
}

// NewVisualizationService creates a new visualization service
func NewVisualizationService(dbManager *dbmanager.Manager) VisualizationService {
	return &visualizationService{
		dbManager: dbManager,
	}
}

// GetTableSuggestions generates visualization suggestions for a specific table
func (s *visualizationService) GetTableSuggestions(ctx context.Context, chatID, tableName string) ([]VisualizationSuggestion, error) {
	// Check if there's an active connection
	if !s.dbManager.IsConnected(chatID) {
		return nil, fmt.Errorf("no active connection for chat %s", chatID)
	}

	// Get schema information
	schemaStorage, err := s.dbManager.GetSchemaWithExamples(ctx, chatID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for chat %s: %v", chatID, err)
	}

	// Find the table in the schema
	if schemaStorage == nil || schemaStorage.FullSchema == nil || schemaStorage.FullSchema.Tables == nil {
		return nil, fmt.Errorf("schema not found or invalid for chat %s", chatID)
	}

	tableSchema, ok := schemaStorage.FullSchema.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in schema", tableName)
	}

	suggestions := generateSuggestions(&tableSchema, tableName)
	return suggestions, nil
}

// GetAllSuggestions generates visualization suggestions for all tables
func (s *visualizationService) GetAllSuggestions(ctx context.Context, chatID string) (map[string][]VisualizationSuggestion, error) {
	// Get schema information
	schemaStorage, err := s.dbManager.GetSchemaWithExamples(ctx, chatID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %v", err)
	}

	if schemaStorage == nil || schemaStorage.FullSchema == nil || schemaStorage.FullSchema.Tables == nil {
		return nil, fmt.Errorf("schema not found or invalid")
	}

	result := make(map[string][]VisualizationSuggestion)

	// Generate suggestions for each table
	for tableName, tableSchema := range schemaStorage.FullSchema.Tables {
		tableSchemaCopy := tableSchema // Create a copy to avoid reference issues
		suggestions := generateSuggestions(&tableSchemaCopy, tableName)
		result[tableName] = suggestions
	}

	return result, nil
}

// ExecuteVisualization executes the query for a visualization and returns the data
func (s *visualizationService) ExecuteVisualization(ctx context.Context, chatID, streamID string, suggestion VisualizationSuggestion) (*VisualizationData, error) {
	// Execute query
	result, qErr := s.dbManager.ExecuteQuery(ctx, chatID, "", "", streamID, suggestion.Query, "SELECT", false)
	if qErr != nil {
		return nil, fmt.Errorf("failed to execute query: %v", qErr.Message)
	}

	// Create visualization data
	visData := &VisualizationData{
		Title:             suggestion.Title,
		Description:       suggestion.Description,
		VisualizationType: suggestion.VisualizationType,
		Query:             suggestion.Query,
		TableName:         suggestion.TableName,
		Data:              result.Result,
		ExecutionTime:     result.ExecutionTime,
	}

	return visData, nil
}

// generateSuggestions creates visualization suggestions based on table schema
func generateSuggestions(tableSchema *dbmanager.TableSchema, tableName string) []VisualizationSuggestion {
	var suggestions []VisualizationSuggestion

	// Check for timestamp/date columns for time series
	var timeColumns []string
	var numericColumns []string
	var categoryColumns []string

	for colName, column := range tableSchema.Columns {
		colType := strings.ToLower(column.Type)

		// Identify time/date columns
		if strings.Contains(colType, "time") || strings.Contains(colType, "date") {
			timeColumns = append(timeColumns, colName)
		}

		// Identify numeric columns
		if strings.Contains(colType, "int") || strings.Contains(colType, "float") ||
			strings.Contains(colType, "decimal") || strings.Contains(colType, "double") ||
			strings.Contains(colType, "numeric") {
			numericColumns = append(numericColumns, colName)
		}

		// Identify category columns
		if strings.Contains(colType, "char") || strings.Contains(colType, "text") ||
			strings.Contains(colType, "enum") || (strings.Contains(colType, "var") && !strings.Contains(colType, "binary")) {
			categoryColumns = append(categoryColumns, colName)
		}
	}

	// Add basic table data suggestion
	suggestions = append(suggestions, VisualizationSuggestion{
		Title:             fmt.Sprintf("%s Overview", formatTableName(tableName)),
		Description:       fmt.Sprintf("Overview of all data in the %s table", formatTableName(tableName)),
		VisualizationType: Table,
		Query:             fmt.Sprintf("SELECT * FROM %s LIMIT 100", tableName),
		TableName:         tableName,
	})

	// Add count suggestion
	suggestions = append(suggestions, VisualizationSuggestion{
		Title:             fmt.Sprintf("Total %s Count", formatTableName(tableName)),
		Description:       fmt.Sprintf("Total number of records in the %s table", formatTableName(tableName)),
		VisualizationType: Table,
		Query:             fmt.Sprintf("SELECT COUNT(*) AS total_count FROM %s", tableName),
		TableName:         tableName,
	})

	// If we have time columns, add time-based suggestions
	for _, timeCol := range timeColumns {
		// Add time series for each numeric column if we have both
		for _, numCol := range numericColumns {
			suggestions = append(suggestions, VisualizationSuggestion{
				Title:             fmt.Sprintf("%s over time", formatColumnName(numCol)),
				Description:       fmt.Sprintf("Time series analysis of %s by %s", formatColumnName(numCol), formatColumnName(timeCol)),
				VisualizationType: TimeSeriesChart,
				Query:             fmt.Sprintf("SELECT %s, %s FROM %s ORDER BY %s", timeCol, numCol, tableName, timeCol),
				TableName:         tableName,
			})
		}

		// Add count by time period
		suggestions = append(suggestions, VisualizationSuggestion{
			Title:             fmt.Sprintf("%s by Date", formatTableName(tableName)),
			Description:       fmt.Sprintf("Count of %s records by date", formatTableName(tableName)),
			VisualizationType: BarChart,
			Query:             fmt.Sprintf("SELECT DATE(%s) as date, COUNT(*) as count FROM %s GROUP BY DATE(%s) ORDER BY date", timeCol, tableName, timeCol),
			TableName:         tableName,
		})

		// Add monthly trend if it's a timestamp
		if strings.Contains(strings.ToLower(timeCol), "time") || strings.Contains(strings.ToLower(timeCol), "date") {
			suggestions = append(suggestions, VisualizationSuggestion{
				Title:             fmt.Sprintf("Monthly %s Trend", formatTableName(tableName)),
				Description:       fmt.Sprintf("Monthly trend of %s records", formatTableName(tableName)),
				VisualizationType: LineChart,
				Query:             fmt.Sprintf("SELECT DATE_TRUNC('month', %s) as month, COUNT(*) as count FROM %s GROUP BY DATE_TRUNC('month', %s) ORDER BY month", timeCol, tableName, timeCol),
				TableName:         tableName,
			})
		}
	}

	// If we have category columns, add distribution suggestions
	for _, catCol := range categoryColumns {
		suggestions = append(suggestions, VisualizationSuggestion{
			Title:             fmt.Sprintf("%s Distribution", formatColumnName(catCol)),
			Description:       fmt.Sprintf("Distribution of records by %s", formatColumnName(catCol)),
			VisualizationType: PieChart,
			Query:             fmt.Sprintf("SELECT %s, COUNT(*) as count FROM %s GROUP BY %s ORDER BY count DESC LIMIT 10", catCol, tableName, catCol),
			TableName:         tableName,
		})
	}

	// If we have multiple numeric columns, suggest comparisons
	if len(numericColumns) >= 2 && len(numericColumns) <= 5 {
		query := fmt.Sprintf("SELECT AVG(%s) as %s", numericColumns[0], numericColumns[0])
		for i := 1; i < len(numericColumns); i++ {
			query += fmt.Sprintf(", AVG(%s) as %s", numericColumns[i], numericColumns[i])
		}
		query += fmt.Sprintf(" FROM %s", tableName)

		suggestions = append(suggestions, VisualizationSuggestion{
			Title:             "Numeric Metrics Comparison",
			Description:       fmt.Sprintf("Comparison of average values for numeric fields in %s", formatTableName(tableName)),
			VisualizationType: BarChart,
			Query:             query,
			TableName:         tableName,
		})
	}

	return suggestions
}

// formatTableName formats a table name to be more readable
func formatTableName(tableName string) string {
	// Convert snake_case to Title Case
	parts := strings.Split(tableName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[0:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

// formatColumnName formats a column name to be more readable
func formatColumnName(columnName string) string {
	// Convert snake_case to Title Case
	parts := strings.Split(columnName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[0:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}
