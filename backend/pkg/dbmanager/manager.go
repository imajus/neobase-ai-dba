package dbmanager

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"neobase-ai/pkg/redis"
)

const (
	cleanupInterval = 2 * time.Minute  // Check every 2 minutes
	idleTimeout     = 10 * time.Minute // Close after 10 minutes of inactivity
)

// DatabaseDriver interface that all database drivers must implement
type DatabaseDriver interface {
	Connect(config ConnectionConfig) (*Connection, error)
	Disconnect(conn *Connection) error
	Ping(conn *Connection) error
	IsAlive(conn *Connection) bool
}

// Manager handles database connections
type Manager struct {
	connections   map[string]*Connection    // chatID -> connection
	drivers       map[string]DatabaseDriver // type -> driver
	mu            sync.RWMutex
	redisRepo     redis.IRedisRepositories
	stopCleanup   chan struct{} // Channel to stop cleanup routine
	eventChan     chan SSEEvent // Channel for SSE events
	schemaManager *SchemaManager
}

// NewManager creates a new connection manager
func NewManager(redisRepo redis.IRedisRepositories, encryptionKey string) (*Manager, error) {
	schemaManager, err := NewSchemaManager(redisRepo, encryptionKey)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		connections:   make(map[string]*Connection),
		drivers:       make(map[string]DatabaseDriver),
		redisRepo:     redisRepo,
		stopCleanup:   make(chan struct{}),
		eventChan:     make(chan SSEEvent, 100),
		schemaManager: schemaManager,
	}

	// Start cleanup routine
	go m.startCleanupRoutine()
	return m, nil
}

// RegisterDriver registers a new database driver
func (m *Manager) RegisterDriver(dbType string, driver DatabaseDriver) {
	m.drivers[dbType] = driver
}

// Connect creates a new database connection
func (m *Manager) Connect(chatID, userID string, config ConnectionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if connection already exists
	if conn, exists := m.connections[chatID]; exists && conn.Status == StatusConnected {
		return fmt.Errorf("connection already exists for chat ID: %s", chatID)
	}

	// Get appropriate driver
	driver, exists := m.drivers[config.Type]
	if !exists {
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}

	// Create connection
	conn, err := driver.Connect(config)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	// Set initial last used time
	conn.LastUsed = time.Now()

	// Store connection
	m.connections[chatID] = conn

	// Cache connection state in Redis
	ctx := context.Background()
	connKey := fmt.Sprintf("conn:%s", chatID)
	pipe := m.redisRepo.StartPipeline(ctx)
	pipe.Set(ctx, connKey, "connected", idleTimeout)
	if err := pipe.Execute(ctx); err != nil {
		log.Printf("Failed to cache connection state: %v", err)
	}

	// Notify subscribers of successful connection
	m.notifySubscribers(chatID, userID, StatusConnected, "")

	return nil
}

// Disconnect closes a database connection
func (m *Manager) Disconnect(chatID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return fmt.Errorf("no connection found for chat ID: %s", chatID)
	}

	// Get driver
	driver, exists := m.drivers[conn.Config.Type]
	if !exists {
		return fmt.Errorf("driver not found for type: %s", conn.Config.Type)
	}

	// Disconnect
	if err := driver.Disconnect(conn); err != nil {
		return fmt.Errorf("failed to disconnect: %v", err)
	}

	// Remove from cache
	ctx := context.Background()
	connKey := fmt.Sprintf("conn:%s", chatID)
	if err := m.redisRepo.Del(connKey, ctx); err != nil {
		log.Printf("Failed to remove connection state from cache: %v", err)
	}

	// Notify subscribers of disconnection
	m.notifySubscribers(chatID, userID, StatusDisconnected, "Connection closed by user")

	delete(m.connections, chatID)
	return nil
}

// GetConnection returns a wrapped connection with usage tracking
func (m *Manager) GetConnection(chatID string) (DBExecutor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return nil, fmt.Errorf("no connection found for chat ID: %s", chatID)
	}

	if conn.Status != StatusConnected {
		return nil, fmt.Errorf("connection is not active")
	}

	// Update last used time
	conn.LastUsed = time.Now()

	// Refresh Redis TTL
	ctx := context.Background()
	connKey := fmt.Sprintf("conn:%s", chatID)
	pipe := m.redisRepo.StartPipeline(ctx)
	pipe.Set(ctx, connKey, "connected", idleTimeout)
	if err := pipe.Execute(ctx); err != nil {
		log.Printf("Failed to refresh connection TTL: %v", err)
	}

	// Return appropriate wrapper based on database type
	switch conn.Config.Type {
	case "postgresql":
		return NewPostgresWrapper(conn.DB, m, chatID), nil
	// Add cases for other database types
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Config.Type)
	}
}

// startCleanupRoutine periodically checks for and closes inactive connections
func (m *Manager) startCleanupRoutine() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanup closes inactive connections
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for chatID, conn := range m.connections {
		if now.Sub(conn.LastUsed) > idleTimeout {
			log.Printf("Closing idle connection for chat %s (last used: %v)",
				chatID, conn.LastUsed.Format(time.RFC3339))

			// Get driver
			driver, exists := m.drivers[conn.Config.Type]
			if !exists {
				log.Printf("Driver not found for type: %s", conn.Config.Type)
				continue
			}

			// Disconnect
			if err := driver.Disconnect(conn); err != nil {
				log.Printf("Failed to disconnect: %v", err)
			}

			// Remove from cache
			ctx := context.Background()
			connKey := fmt.Sprintf("conn:%s", chatID)
			if err := m.redisRepo.Del(connKey, ctx); err != nil {
				log.Printf("Failed to remove connection state from cache: %v", err)
			}

			// Notify subscribers of disconnection
			m.notifySubscribers(chatID, "", StatusDisconnected, "Connection closed due to inactivity")

			delete(m.connections, chatID)
			log.Printf("Successfully closed idle connection for chat %s", chatID)
		}
	}
}

// Stop gracefully stops the manager and cleans up resources
func (m *Manager) Stop() error {
	// Signal cleanup routine to stop
	close(m.stopCleanup)

	// Close all active connections
	m.mu.Lock()
	defer m.mu.Unlock()

	for chatID, conn := range m.connections {
		driver, exists := m.drivers[conn.Config.Type]
		if !exists {
			continue
		}

		if err := driver.Disconnect(conn); err != nil {
			log.Printf("Failed to disconnect chat %s: %v", chatID, err)
		}

		ctx := context.Background()
		connKey := fmt.Sprintf("conn:%s", chatID)
		if err := m.redisRepo.Del(connKey, ctx); err != nil {
			log.Printf("Failed to remove connection state from cache for chat %s: %v", chatID, err)
		}
	}

	// Clear connections map
	m.connections = make(map[string]*Connection)
	return nil
}

// UpdateLastUsed updates the last used timestamp for a connection
func (m *Manager) UpdateLastUsed(chatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return fmt.Errorf("no connection found for chat ID: %s", chatID)
	}

	conn.LastUsed = time.Now()

	// Refresh Redis TTL
	ctx := context.Background()
	connKey := fmt.Sprintf("conn:%s", chatID)
	pipe := m.redisRepo.StartPipeline(ctx)
	pipe.Set(ctx, connKey, "connected", idleTimeout)
	if err := pipe.Execute(ctx); err != nil {
		log.Printf("Failed to refresh connection TTL: %v", err)
	}

	return nil
}

// Add subscriber to connection
func (m *Manager) Subscribe(chatID, userID, streamID string) error {
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no connection found for chat ID: %s", chatID)
	}

	conn.SubLock.Lock()
	if conn.Subscribers == nil {
		conn.Subscribers = make(map[string]bool)
	}
	conn.Subscribers[streamID] = true
	conn.SubLock.Unlock()

	// Send current status to new subscriber
	m.eventChan <- SSEEvent{
		UserID:    userID,
		ChatID:    chatID,
		StreamID:  streamID,
		Status:    conn.Status,
		Timestamp: time.Now(),
		Error:     conn.Error,
	}

	return nil
}

// Remove subscriber
func (m *Manager) Unsubscribe(chatID, deviceID string) {
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	conn.SubLock.Lock()
	delete(conn.Subscribers, deviceID)
	conn.SubLock.Unlock()
}

// Get event channel for SSE
func (m *Manager) GetEventChannel() <-chan SSEEvent {
	return m.eventChan
}

// Notify subscribers of connection status change
func (m *Manager) notifySubscribers(chatID, userID string, status ConnectionStatus, err string) {
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	conn.SubLock.RLock()
	defer conn.SubLock.RUnlock()

	// Send to all subscribers
	for streamID := range conn.Subscribers {
		event := SSEEvent{
			UserID:    userID,
			ChatID:    chatID,
			StreamID:  streamID,
			Status:    status,
			Timestamp: time.Now(),
			Error:     err,
		}

		select {
		case m.eventChan <- event:
		default:
			log.Printf("Warning: Event channel full, dropped event for chat %s stream %s", chatID, streamID)
		}
	}
}

func (m *Manager) StartSchemaTracking(chatID string) {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				conn, err := m.GetConnection(chatID)
				if err != nil {
					continue
				}

				// Get connection type from the connections map
				m.mu.RLock()
				dbConn, exists := m.connections[chatID]
				m.mu.RUnlock()
				if !exists {
					continue
				}

				diff, err := m.schemaManager.CheckSchemaChanges(context.Background(), chatID, conn, dbConn.Config.Type)
				if err != nil {
					log.Printf("Error checking schema changes: %v", err)
					continue
				}

				if diff != nil {
					log.Printf("Schema changes detected for chat %s: %v", chatID, diff)
				}
			case <-m.stopCleanup:
				return
			}
		}
	}()
}
