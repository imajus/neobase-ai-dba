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
	IsActive bool    `bson:"is_active" json:"is_active"`
	Base     `bson:",inline"`
}

type Chat struct {
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	Connection Connection         `bson:"connection" json:"connection"`
	IsActive   bool               `bson:"is_active" json:"is_active"`
	Base       `bson:",inline"`
}

func NewChat(userID primitive.ObjectID, connection Connection) *Chat {
	return &Chat{
		UserID:     userID,
		Connection: connection,
		IsActive:   true,
		Base:       NewBase(),
	}
}
