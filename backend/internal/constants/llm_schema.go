package constants

// LLMResponse represents the structured response from LLM
type LLMResponse struct {
	Queries          []QueryInfo    `json:"queries,omitempty"`
	AssistantMessage string         `json:"assistantMessage"`
	ActionButtons    []ActionButton `json:"actionButtons,omitempty"`
}

// ActionButton represents a UI action button that can be suggested by the LLM
type ActionButton struct {
	Label     string `json:"label"`     // Display text for the button
	Action    string `json:"action"`    // Action identifier (e.g., "refresh_schema", "show_tables")
	IsPrimary bool   `json:"isPrimary"` // Whether this is a primary (highlighted) action
}

// QueryInfo represents a single query in the LLM response
type QueryInfo struct {
	Query                  string                    `json:"query"`
	Tables                 *string                   `json:"tables,omitempty"`
	Collection             *string                   `json:"collection,omitempty"`
	QueryType              string                    `json:"queryType"`
	Pagination             *Pagination               `json:"pagination,omitempty"`
	IsCritical             bool                      `json:"isCritical"`
	CanRollback            bool                      `json:"canRollback"`
	Explanation            string                    `json:"explanation"`
	ExampleResultString    *string                   `json:"exampleResultString"`
	ExampleResult          *[]map[string]interface{} `json:"exampleResult,omitempty"`
	RollbackQuery          string                    `json:"rollbackQuery,omitempty"`
	EstimateResponseTime   interface{}               `json:"estimateResponseTime"`
	RollbackDependentQuery string                    `json:"rollbackDependentQuery,omitempty"`
}

type Pagination struct {
	TotalRecordsCount *int    `json:"total_records_count"`
	PaginatedQuery    *string `json:"paginated_query"`
}
