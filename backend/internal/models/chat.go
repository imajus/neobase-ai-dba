package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Connection struct {
	Type     string  `bson:"type" json:"type"`
	Host     string  `bson:"host" json:"host"`
	Port     string  `bson:"port" json:"port"`
	Username *string `bson:"username" json:"username"`
	Password *string `bson:"password" json:"-"` // Hide in JSON
	Database string  `bson:"database" json:"database"`

	// SSL/TLS Configuration
	UseSSL         bool    `bson:"use_ssl" json:"use_ssl"`
	SSLCertURL     *string `bson:"ssl_cert_url,omitempty" json:"ssl_cert_url,omitempty"`
	SSLKeyURL      *string `bson:"ssl_key_url,omitempty" json:"ssl_key_url,omitempty"`
	SSLRootCertURL *string `bson:"ssl_root_cert_url,omitempty" json:"ssl_root_cert_url,omitempty"`

	Base `bson:",inline"`
}

type Chat struct {
	UserID              primitive.ObjectID `bson:"user_id" json:"user_id"`
	Connection          Connection         `bson:"connection" json:"connection"`
	SelectedCollections string             `bson:"selected_collections" json:"selected_collections"` // "ALL" or comma-separated table names
	AutoExecuteQuery    bool               `bson:"auto_execute_query" json:"auto_execute_query"`     // default is false, Execute query automatically when LLM response is received
	Base                `bson:",inline"`
}

func NewChat(userID primitive.ObjectID, connection Connection, autoExecuteQuery bool) *Chat {
	return &Chat{
		UserID:              userID,
		Connection:          connection,
		AutoExecuteQuery:    autoExecuteQuery,
		SelectedCollections: "ALL", // Default to ALL tables
		Base:                NewBase(),
	}
}
