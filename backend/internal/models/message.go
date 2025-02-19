package models

import (
	"log"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	UserID  primitive.ObjectID `bson:"user_id" json:"user_id"`
	ChatID  primitive.ObjectID `bson:"chat_id" json:"chat_id"`
	Type    string             `bson:"type" json:"type"` // 'user' or 'assistant'
	Content string             `bson:"content" json:"content"`
	Queries *[]Query           `bson:"queries,omitempty" json:"queries,omitempty"`
	Base    `bson:",inline"`
}

type Query struct {
	ID                     primitive.ObjectID `bson:"id" json:"id"`
	Query                  string             `bson:"query" json:"query"`
	QueryType              *string            `bson:"query_type" json:"query_type"` // SELECT, INSERT, UPDATE, DELETE...
	Tables                 *string            `bson:"tables" json:"tables"`         // comma separated table names involved in the query
	Description            string             `bson:"description" json:"description"`
	RollbackDependentQuery *string            `bson:"rollback_dependent_query,omitempty" json:"rollback_dependent_query,omitempty"` // ID of the query that this query depends on
	RollbackQuery          *string            `bson:"rollback_query,omitempty" json:"rollback_query,omitempty"`                     // the query to rollback the query
	ExecutionTime          *int               `bson:"execution_time" json:"execution_time"`                                         // in milliseconds, same for execution & rollback query
	ExampleExecutionTime   int                `bson:"example_execution_time" json:"example_execution_time"`                         // in milliseconds
	CanRollback            bool               `bson:"can_rollback" json:"can_rollback"`
	IsCritical             bool               `bson:"is_critical" json:"is_critical"`
	IsExecuted             bool               `bson:"is_executed" json:"is_executed"`       // if the query has been executed
	IsRolledBack           bool               `bson:"is_rolled_back" json:"is_rolled_back"` // if the query has been rolled back
	Error                  *QueryError        `bson:"error,omitempty" json:"error,omitempty"`
	ExampleResult          *string            `bson:"example_result,omitempty" json:"example_result,omitempty"`     // JSON string
	ExecutionResult        *string            `bson:"execution_result,omitempty" json:"execution_result,omitempty"` // JSON string
}

type QueryError struct {
	Code    string `bson:"code" json:"code"`
	Message string `bson:"message" json:"message"`
	Details string `bson:"details" json:"details"`
}

func NewMessage(userID, chatID primitive.ObjectID, msgType, content string, queries *[]Query) *Message {
	log.Printf("NewMessage -> queries: %v", queries)
	return &Message{
		UserID:  userID,
		ChatID:  chatID,
		Type:    msgType,
		Content: content,
		Queries: queries,
		Base:    NewBase(),
	}
}
