package dtos

import (
	"encoding/json"
	"neobase-ai/internal/models"
)

type CreateMessageRequest struct {
	StreamID string `json:"stream_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

type MessageResponse struct {
	ID        string                 `json:"id"`
	ChatID    string                 `json:"chat_id"`
	Type      string                 `json:"type"`
	Content   map[string]interface{} `json:"content"` // Should be a map[string]interface{}
	Queries   *[]Query               `json:"queries,omitempty"`
	CreatedAt string                 `json:"created_at"`
}

type Query struct {
	ID              string                 `json:"id"`
	Query           string                 `json:"query"`
	Description     string                 `json:"description"`
	ExecutionTime   int                    `json:"execution_time"`
	CanRollback     bool                   `json:"can_rollback"`
	IsCritical      bool                   `json:"is_critical"`
	IsExecuted      bool                   `json:"is_executed"`
	IsRolledBack    bool                   `json:"is_rolled_back"`
	Error           *QueryError            `json:"error,omitempty"`
	ExampleResult   map[string]interface{} `json:"example_result,omitempty"`
	ExecutionResult map[string]interface{} `json:"execution_result,omitempty"`
}

type QueryError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details"`
}

type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
	Total    int64             `json:"total"`
}

type MessageListRequest struct {
	ChatID   string `form:"chat_id" binding:"required"`
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"page_size" binding:"required,min=1,max=100"`
}

func ToQueryDto(queries *[]models.Query) *[]Query {
	if queries == nil {
		return nil
	}
	queriesDto := make([]Query, len(*queries))
	for i, query := range *queries {
		var exampleResult map[string]interface{}
		var executionResult map[string]interface{}
		err := json.Unmarshal([]byte(*query.ExampleResult), &exampleResult)
		if err != nil {
			exampleResult = map[string]interface{}{}
		}
		err = json.Unmarshal([]byte(*query.ExecutionResult), &executionResult)
		if err != nil {
			executionResult = map[string]interface{}{}
		}
		queriesDto[i] = Query{
			ID:              query.ID.Hex(),
			Query:           query.Query,
			Description:     query.Description,
			ExecutionTime:   query.ExecutionTime,
			CanRollback:     query.CanRollback,
			IsCritical:      query.IsCritical,
			IsExecuted:      query.IsExecuted,
			IsRolledBack:    query.IsRolledBack,
			Error:           (*QueryError)(query.Error),
			ExampleResult:   exampleResult,
			ExecutionResult: executionResult,
		}
	}
	return &queriesDto
}
