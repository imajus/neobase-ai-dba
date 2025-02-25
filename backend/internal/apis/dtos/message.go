package dtos

import (
	"encoding/json"
	"log"
	"neobase-ai/internal/models"
)

type CreateMessageRequest struct {
	StreamID string `json:"stream_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

type MessageResponse struct {
	ID        string   `json:"id"`
	ChatID    string   `json:"chat_id"`
	Type      string   `json:"type"`
	Content   string   `json:"content"`
	Queries   *[]Query `json:"queries,omitempty"`
	CreatedAt string   `json:"created_at"`
}

type Query struct {
	ID                     string                 `json:"id"`
	Query                  string                 `json:"query"`
	Description            string                 `json:"description"`
	ExecutionTime          *int                   `json:"execution_time,omitempty"`
	ExampleExecutionTime   int                    `json:"example_execution_time"`
	CanRollback            bool                   `json:"can_rollback"`
	IsCritical             bool                   `json:"is_critical"`
	IsExecuted             bool                   `json:"is_executed"`
	IsRolledBack           bool                   `json:"is_rolled_back"`
	Error                  *QueryError            `json:"error,omitempty"`
	ExampleResult          []interface{}          `json:"example_result,omitempty"`
	ExecutionResult        map[string]interface{} `json:"execution_result,omitempty"`
	QueryType              *string                `json:"query_type,omitempty"`
	Tables                 *string                `json:"tables,omitempty"`
	RollbackQuery          *string                `json:"rollback_query,omitempty"`
	RollbackDependentQuery *string                `json:"rollback_dependent_query,omitempty"`
	Pagination             *Pagination            `json:"pagination,omitempty"`
}

type Pagination struct {
	TotalRecordsCount int `json:"total_records_count"`
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
		var exampleResult []interface{}
		var executionResult map[string]interface{}

		if query.ExampleResult != nil {
			log.Printf("ToQueryDto -> query.ExampleResult: %v", *query.ExampleResult)
			err := json.Unmarshal([]byte(*query.ExampleResult), &exampleResult)
			if err != nil {
				log.Printf("ToQueryDto -> error unmarshalling exampleResult: %v", err)
				exampleResult = []interface{}{}
			}
		}

		if query.ExecutionResult != nil {
			err := json.Unmarshal([]byte(*query.ExecutionResult), &executionResult)
			if err != nil {
				executionResult = map[string]interface{}{}
			}
		}

		var pagination *Pagination
		if query.Pagination != nil {
			totalCount := 0
			if query.Pagination.TotalRecordsCount != nil {
				totalCount = *query.Pagination.TotalRecordsCount
			}
			pagination = &Pagination{
				TotalRecordsCount: totalCount,
			}
		}
		log.Printf("ToQueryDto -> final exampleResult: %v", exampleResult)
		queriesDto[i] = Query{
			ID:                     query.ID.Hex(),
			Query:                  query.Query,
			Description:            query.Description,
			ExecutionTime:          query.ExecutionTime,
			ExampleExecutionTime:   query.ExampleExecutionTime,
			CanRollback:            query.CanRollback,
			IsCritical:             query.IsCritical,
			IsExecuted:             query.IsExecuted,
			IsRolledBack:           query.IsRolledBack,
			Error:                  (*QueryError)(query.Error),
			ExampleResult:          exampleResult,
			ExecutionResult:        executionResult,
			QueryType:              query.QueryType,
			Tables:                 query.Tables,
			RollbackQuery:          query.RollbackQuery,
			RollbackDependentQuery: query.RollbackDependentQuery,
			Pagination:             pagination,
		}
	}
	return &queriesDto
}
