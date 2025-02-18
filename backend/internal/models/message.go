package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	UserID  primitive.ObjectID `bson:"user_id" json:"user_id"`
	ChatID  primitive.ObjectID `bson:"chat_id" json:"chat_id"`
	Type    string             `bson:"type" json:"type"` // 'user' or 'ai'
	Content string             `bson:"content" json:"content"`
	Queries *[]Query           `bson:"queries,omitempty" json:"queries,omitempty"`
	Base    `bson:",inline"`
}

type Query struct {
	ID              primitive.ObjectID `bson:"id" json:"id"`
	Query           string             `bson:"query" json:"query"`
	Description     string             `bson:"description" json:"description"`
	ExecutionTime   int                `bson:"execution_time" json:"execution_time"` // in milliseconds
	CanRollback     bool               `bson:"can_rollback" json:"can_rollback"`
	IsCritical      bool               `bson:"is_critical" json:"is_critical"`
	IsExecuted      bool               `bson:"is_executed" json:"is_executed"`       // if the query has been executed
	IsRolledBack    bool               `bson:"is_rolled_back" json:"is_rolled_back"` // if the query has been rolled back
	Error           *QueryError        `bson:"error,omitempty" json:"error,omitempty"`
	ExampleResult   *string            `bson:"example_result,omitempty" json:"example_result,omitempty"`     // JSON string
	ExecutionResult *string            `bson:"execution_result,omitempty" json:"execution_result,omitempty"` // JSON string
}

type QueryError struct {
	Code    string `bson:"code" json:"code"`
	Message string `bson:"message" json:"message"`
	Details string `bson:"details" json:"details"`
}

func NewMessage(userID, chatID primitive.ObjectID, msgType, content string, queries *[]Query) *Message {
	return &Message{
		UserID:  userID,
		ChatID:  chatID,
		Type:    msgType,
		Content: content,
		Queries: queries,
		Base:    NewBase(),
	}
}
