package dbmanager

import (
	"sync"
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
	DB          *gorm.DB
	LastUsed    time.Time
	Status      ConnectionStatus
	Error       string
	Config      ConnectionConfig
	Subscribers map[string]bool // Map of subscriber IDs (e.g., deviceIDs) that need notifications
	SubLock     sync.RWMutex    // Lock for thread-safe subscriber operations
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

// SSEEvent represents an event to be sent via SSE
type SSEEvent struct {
	UserID    string           `json:"user_id"`
	ChatID    string           `json:"chat_id"`
	StreamID  string           `json:"stream_id"`
	Status    ConnectionStatus `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Error     string           `json:"error,omitempty"`
}
