package dtos

type ExecuteQueryRequest struct {
	MessageID string `json:"message_id" binding:"required"`
	QueryID   string `json:"query_id" binding:"required"`
	StreamID  string `json:"stream_id" binding:"required"`
}

type RollbackQueryRequest struct {
	MessageID string `json:"message_id" binding:"required"`
	QueryID   string `json:"query_id" binding:"required"`
	StreamID  string `json:"stream_id" binding:"required"`
}

type CancelQueryExecutionRequest struct {
	MessageID string `json:"message_id" binding:"required"`
	QueryID   string `json:"query_id" binding:"required"`
	StreamID  string `json:"stream_id" binding:"required"`
}

type QueryExecutionResponse struct {
	ChatID          string                 `json:"chat_id"`
	MessageID       string                 `json:"message_id"`
	QueryID         string                 `json:"query_id"`
	IsExecuted      bool                   `json:"is_executed"`
	IsRolledBack    bool                   `json:"is_rolled_back"`
	ExecutionTime   int                    `json:"execution_time"`
	ExecutionResult map[string]interface{} `json:"execution_result"`
	Error           *QueryError            `json:"error,omitempty"`
}
