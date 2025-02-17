package dbmanager

import (
	"time"

	"gorm.io/gorm"
)

// ConnectionStatus represents the current state of a database connection
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusError        ConnectionStatus = "error"
)

// Connection represents an active database connection
type Connection struct {
	DB       *gorm.DB
	LastUsed time.Time
	Status   ConnectionStatus
	Error    string
	Config   ConnectionConfig
}

// ConnectionConfig holds the configuration for a database connection
type ConnectionConfig struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}
