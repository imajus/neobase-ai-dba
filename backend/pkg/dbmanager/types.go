package dbmanager

import (
	"neobase-ai/internal/apis/dtos"
	"sync"
	"time"

	"gorm.io/gorm"
)

// ConnectionStatus represents the current state of a database connection
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "db-connected"
	StatusDisconnected ConnectionStatus = "db-disconnected"
	StatusError        ConnectionStatus = "db-error"
)

// Connection represents an active database connection
type Connection struct {
	DB          *gorm.DB
	LastUsed    time.Time
	Status      ConnectionStatus
	Error       string
	Config      ConnectionConfig
	UserID      string
	ChatID      string
	StreamID    string
	Subscribers map[string]bool // Map of subscriber IDs (e.g., streamIDs) that need notifications
	SubLock     sync.RWMutex    // Lock for thread-safe subscriber operations
}

// ConnectionConfig holds the configuration for a database connection
type ConnectionConfig struct {
	Type     string  `json:"type"`
	Host     string  `json:"host"`
	Port     string  `json:"port"`
	Username *string `json:"username"`
	Password *string `json:"password"`
	Database string  `json:"database"`
}

// SSEEvent represents an event to be sent via SSE
type SSEEvent struct {
	UserID    string           `json:"user_id"`
	ChatID    string           `json:"chat_id"`
	StreamID  string           `json:"stream_id"`
	Status    ConnectionStatus `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Error     string           `json:"error,omitempty"`
}

// StreamHandler interface for handling database events
type StreamHandler interface {
	HandleDBEvent(userID, chatID, streamID string, response dtos.StreamResponse)
}
