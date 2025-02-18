package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"neobase-ai/internal/apis/dtos"
	"neobase-ai/pkg/redis"
)

const (
	cleanupInterval = 2 * time.Minute  // Check every 30 seconds
	idleTimeout     = 10 * time.Minute // Close after 10 seconds of inactivity
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
	streamHandler StreamHandler // Changed from *StreamHandler to StreamHandler
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
	log.Printf("DBManager -> Registered driver for type: %s", dbType)
}

// Connect creates a new database connection
func (m *Manager) Connect(chatID, userID, streamID string, config ConnectionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("DBManager -> Connect -> Starting connection for chatID: %s", chatID)

	// Get existing subscribers if connection exists
	var existingSubscribers map[string]bool
	if existingConn, exists := m.connections[chatID]; exists {
		existingConn.SubLock.RLock()
		existingSubscribers = make(map[string]bool)
		for id := range existingConn.Subscribers {
			existingSubscribers[id] = true
		}
		existingConn.SubLock.RUnlock()
		log.Printf("DBManager -> Connect -> Preserving existing subscribers: %+v", existingSubscribers)
	}

	// Get appropriate driver
	driver, exists := m.drivers[config.Type]
	if !exists {
		log.Printf("DBManager -> Connect -> No driver found for type: %s", config.Type)
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}

	log.Printf("DBManager -> Connect -> Found driver for type: %s", config.Type)

	// Create connection
	conn, err := driver.Connect(config)
	if err != nil {
		log.Printf("DBManager -> Connect -> Driver connection failed: %v", err)
		return fmt.Errorf("failed to connect: %v", err)
	}

	log.Printf("DBManager -> Connect -> Driver connection successful")

	// Initialize connection fields
	conn.LastUsed = time.Now()
	conn.Status = StatusConnected
	conn.Config = config
	conn.UserID = userID
	conn.ChatID = chatID
	conn.StreamID = streamID

	// Initialize subscribers map with existing subscribers
	conn.Subscribers = make(map[string]bool)
	if existingSubscribers != nil {
		for id := range existingSubscribers {
			conn.Subscribers[id] = true
		}
	}
	// Add current streamID if not already present
	conn.Subscribers[streamID] = true

	log.Printf("DBManager -> Connect -> Initialized subscribers: %+v", conn.Subscribers)

	// Store connection
	m.connections[chatID] = conn
	log.Printf("DBManager -> Connect -> Stored connection in manager")

	// Notify subscribers in a separate goroutine
	go func() {
		m.notifySubscribers(chatID, userID, StatusConnected, "")
		log.Printf("DBManager -> Connect -> Notified subscribers")
	}()

	// Start background tasks in a separate goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("DBManager -> Connect -> Background task panic recovered: %v", r)
			}
		}()

		// Cache connection state in Redis
		ctx := context.Background()
		connKey := fmt.Sprintf("conn:%s", chatID)
		pipe := m.redisRepo.StartPipeline(ctx)
		pipe.Set(ctx, connKey, "connected", idleTimeout)
		if err := pipe.Execute(ctx); err != nil {
			log.Printf("DBManager -> Connect -> Failed to cache connection state: %v", err)
		} else {
			log.Printf("DBManager -> Connect -> Connection state cached in Redis")
		}

		// Start schema tracking
		m.StartSchemaTracking(chatID)
	}()

	log.Printf("DBManager -> Connect -> Connection completed successfully for chatID: %s", chatID)
	return nil
}

// Disconnect closes a database connection
func (m *Manager) Disconnect(chatID, userID string) error {
	log.Printf("DBManager -> Disconnect -> Starting disconnection for chatID: %s", chatID)
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return fmt.Errorf("no connection found for chat ID: %s", chatID)
	}

	log.Printf("DBManager -> Disconnect -> Connection found for chatID: %s", chatID)

	// Only attempt to disconnect if there's an active connection
	if conn.DB != nil {
		// Get driver
		driver, exists := m.drivers[conn.Config.Type]
		if !exists {
			return fmt.Errorf("driver not found for type: %s", conn.Config.Type)
		}

		// Disconnect
		if err := driver.Disconnect(conn); err != nil {
			return fmt.Errorf("failed to disconnect: %v", err)
		}

		log.Printf("DBManager -> Disconnect -> Disconnected from chatID: %s", chatID)

		// Remove from cache
		ctx := context.Background()
		connKey := fmt.Sprintf("conn:%s", chatID)
		if err := m.redisRepo.Del(connKey, ctx); err != nil {
			log.Printf("DBManager -> Disconnect -> Failed to remove connection state from cache: %v", err)
		}
	}

	// Store subscribers before deleting connection
	subscribers := make(map[string]bool)
	conn.SubLock.RLock()
	log.Printf("DBManager -> Disconnect -> Current subscribers: %+v", conn.Subscribers)
	for id := range conn.Subscribers {
		subscribers[id] = true
	}
	conn.SubLock.RUnlock()

	// Delete the connection
	delete(m.connections, chatID)
	log.Printf("DBManager -> Disconnect -> Deleted connection from manager")

	// Notify subscribers after releasing the lock
	if len(subscribers) > 0 {
		go func(subs map[string]bool) {
			log.Printf("DBManager -> Disconnect -> Notifying subscribers: %+v", subs)
			for streamID := range subs {
				response := dtos.StreamResponse{
					Event: string(StatusDisconnected),
					Data:  "Connection closed by user",
				}

				if m.streamHandler != nil {
					log.Printf("DBManager -> Disconnect -> Going to notify subscriber %s of disconnection", streamID)
					m.streamHandler.HandleDBEvent(userID, chatID, streamID, response)
					log.Printf("DBManager -> Disconnect -> Notified subscriber %s of disconnection", streamID)
				}
			}
		}(subscribers)
	} else {
		log.Printf("DBManager -> Disconnect -> No subscribers to notify")
	}

	log.Printf("DBManager -> Disconnect -> Successfully disconnected chat %s", chatID)
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
		if now.Sub(conn.LastUsed) < idleTimeout {
			continue
		}

		log.Printf("DBManager -> cleanup -> Found idle connection for chatID: %s", chatID)

		// Store subscribers before cleanup
		subscribers := make(map[string]bool)
		conn.SubLock.RLock()
		for id := range conn.Subscribers {
			subscribers[id] = true
		}
		// Use stored fields
		userID := conn.UserID
		streamID := conn.StreamID
		conn.SubLock.RUnlock()

		// Get driver and disconnect
		if conn.DB != nil {
			if driver, exists := m.drivers[conn.Config.Type]; exists {
				if err := driver.Disconnect(conn); err != nil {
					log.Printf("DBManager -> cleanup -> Failed to disconnect: %v", err)
					continue
				}
			}

			// Remove from cache
			ctx := context.Background()
			connKey := fmt.Sprintf("conn:%s", chatID)
			if err := m.redisRepo.Del(connKey, ctx); err != nil {
				log.Printf("DBManager -> cleanup -> Failed to remove connection state from cache: %v", err)
			}
		}

		// Delete the connection
		delete(m.connections, chatID)
		log.Printf("DBManager -> cleanup -> Removed idle connection for chatID: %s", chatID)

		log.Printf("DBManager -> cleanup -> Subscribers: %+v", subscribers)
		// Notify subscribers in a separate goroutine
		if len(subscribers) > 0 {
			go func(chatID, userID, streamID string, subs map[string]bool) {
				log.Printf("DBManager -> cleanup -> Notifying %d subscribers for chatID: %s", len(subs), chatID)
				for subStreamID := range subs {
					response := dtos.StreamResponse{
						Event: string(StatusDisconnected),
						Data:  "Connection closed due to inactivity",
					}

					if m.streamHandler != nil {
						log.Printf("DBManager -> cleanup -> Going to notify subscriber %s of cleanup disconnection", subStreamID)
						m.streamHandler.HandleDBEvent(userID, chatID, subStreamID, response)
						log.Printf("DBManager -> cleanup -> Notified subscriber %s of cleanup disconnection", subStreamID)
					}
				}
			}(chatID, userID, streamID, subscribers)
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

// Subscribe adds a subscriber for connection status updates
func (m *Manager) Subscribe(chatID, streamID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("DBManager -> Subscribe -> Adding subscriber %s for chatID: %s", streamID, chatID)

	conn, exists := m.connections[chatID]
	if !exists {
		// Create a placeholder connection for subscribers
		conn = &Connection{
			Subscribers: make(map[string]bool),
			Status:      StatusDisconnected,
			ChatID:      chatID,
			StreamID:    streamID,
			// UserID will be set when actual connection is established
		}
		m.connections[chatID] = conn
		log.Printf("DBManager -> Subscribe -> Created new connection entry for chatID: %s", chatID)
	} else {
		// Update StreamID if connection exists
		conn.StreamID = streamID
	}

	conn.SubLock.Lock()
	if conn.Subscribers == nil {
		conn.Subscribers = make(map[string]bool)
	}
	conn.Subscribers[streamID] = true
	conn.SubLock.Unlock()

	log.Printf("DBManager -> Subscribe -> Added subscriber %s for chatID: %s, total subscribers: %d",
		streamID, chatID, len(conn.Subscribers))
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
	log.Printf("DBManager -> notifySubscribers -> Notifying subscribers for chatID: %s", chatID)

	// Get connection and subscribers under read lock
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("DBManager -> notifySubscribers -> No connection found for chatID: %s", chatID)
		return
	}

	// Get a snapshot of subscribers under read lock
	conn.SubLock.RLock()
	subscribers := make(map[string]bool, len(conn.Subscribers))
	for id := range conn.Subscribers {
		subscribers[id] = true
	}
	conn.SubLock.RUnlock()

	log.Printf("DBManager -> notifySubscribers -> Notifying %d subscribers for chatID: %s", len(subscribers), chatID)

	// Notify subscribers without holding any locks
	for streamID := range subscribers {
		response := dtos.StreamResponse{
			Event: string(status),
			Data:  err,
		}

		if m.streamHandler != nil {
			m.streamHandler.HandleDBEvent(userID, chatID, streamID, response)
			log.Printf("DBManager -> notifySubscribers -> Sent event to stream handler: %+v", response)
		}
	}
}

func (m *Manager) StartSchemaTracking(chatID string) {
	log.Printf("DBManager -> StartSchemaTracking -> Starting for chatID: %s", chatID)

	go func() {
		// Initial delay to let connection stabilize
		time.Sleep(2 * time.Second)

		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		// Do initial schema check
		if err := m.doSchemaCheck(chatID); err != nil {
			log.Printf("DBManager -> StartSchemaTracking -> Initial schema check failed: %v", err)
		}

		for {
			select {
			case <-ticker.C:
				if err := m.doSchemaCheck(chatID); err != nil {
					log.Printf("DBManager -> StartSchemaTracking -> Schema check failed: %v", err)
				}
			case <-m.stopCleanup:
				log.Printf("DBManager -> StartSchemaTracking -> Stopping for chatID: %s", chatID)
				return
			}
		}
	}()
}

func (m *Manager) doSchemaCheck(chatID string) error {
	conn, err := m.GetConnection(chatID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %v", err)
	}

	m.mu.RLock()
	dbConn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found")
	}

	diff, err := m.schemaManager.CheckSchemaChanges(context.Background(), chatID, conn, dbConn.Config.Type)
	if err != nil {
		return fmt.Errorf("schema check failed: %v", err)
	}

	if diff != nil {
		log.Printf("DBManager -> doSchemaCheck -> Schema changes detected for chat %s: %+v", chatID, diff)
	}

	return nil
}

// Add exported methods to access internal fields
func (m *Manager) GetConnections() map[string]*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections
}

func (m *Manager) GetSchemaManager() *SchemaManager {
	return m.schemaManager
}

func (m *Manager) GetConnectionInfo(chatID string) (*ConnectionInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return nil, false
	}

	// Convert Connection to ConnectionInfo
	connInfo := &ConnectionInfo{
		Config: conn.Config,
	}

	// Get the underlying *sql.DB from gorm.DB
	if conn.DB != nil {
		sqlDB, err := conn.DB.DB()
		if err == nil {
			connInfo.DB = sqlDB
		}
	}

	return connInfo, true
}

// IsConnected checks if there is an active connection for the given chat
func (m *Manager) IsConnected(chatID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[chatID]
	if !exists {
		return false
	}

	// Try a simple ping to check if connection is alive
	if conn.DB != nil {
		sqlDB, err := conn.DB.DB()
		if err != nil {
			return false
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = sqlDB.PingContext(ctx)
		return err == nil
	}

	return false
}

type ConnectionInfo struct {
	DB     *sql.DB
	Config ConnectionConfig
}

// SetStreamHandler sets the stream handler for database events
func (m *Manager) SetStreamHandler(handler StreamHandler) {
	m.streamHandler = handler
}
