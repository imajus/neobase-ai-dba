package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	// Database drivers
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	_ "github.com/lib/pq"              // PostgreSQL/YugabyteDB Driver

	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/pkg/redis"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	cleanupInterval     = 10 * time.Minute // Check every 10 minutes
	idleTimeout         = 15 * time.Minute // Close after 15 minutes of inactivity
	schemaCheckInterval = 24 * time.Hour   // Check every 24 hour
)

type cleanupMetrics struct {
	lastRun            time.Time
	connectionsRemoved int
	executionsRemoved  int
}

// Manager handles database connections
type Manager struct {
	connections      map[string]*Connection    // chatID -> connection
	drivers          map[string]DatabaseDriver // type -> driver
	mu               sync.RWMutex
	redisRepo        redis.IRedisRepositories
	stopCleanup      chan struct{} // Channel to stop cleanup routine
	eventChan        chan SSEEvent // Channel for SSE events
	schemaManager    *SchemaManager
	streamHandler    StreamHandler              // Changed from *StreamHandler to StreamHandler
	activeExecutions map[string]*QueryExecution // key: streamID
	executionMu      sync.RWMutex
	cleanupMetrics   cleanupMetrics
	fetchers         map[string]FetcherFactory
	fetchersMu       sync.RWMutex
}

// NewManager creates a new connection manager
func NewManager(redisRepo redis.IRedisRepositories, encryptionKey string) (*Manager, error) {
	schemaManager, err := NewSchemaManager(redisRepo, encryptionKey, nil)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		connections:      make(map[string]*Connection),
		drivers:          make(map[string]DatabaseDriver),
		redisRepo:        redisRepo,
		stopCleanup:      make(chan struct{}),
		eventChan:        make(chan SSEEvent, 100),
		schemaManager:    schemaManager,
		activeExecutions: make(map[string]*QueryExecution),
		executionMu:      sync.RWMutex{},
		fetchers:         make(map[string]FetcherFactory),
	}

	// Set the DBManager in the SchemaManager
	schemaManager.SetDBManager(m)

	// Start cleanup routine in a separate goroutine with error handling
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("DBManager -> Cleanup routine panic recovered: %v", r)
				// Restart the cleanup routine
				go m.startCleanupRoutine()
			}
		}()
		m.startCleanupRoutine()
	}()

	// Register default fetchers
	m.RegisterFetcher("postgresql", func(db DBExecutor) SchemaFetcher {
		return &PostgresDriver{}
	})

	m.RegisterFetcher("yugabytedb", func(db DBExecutor) SchemaFetcher {
		return &PostgresDriver{}
	})

	// Add MySQL schema fetcher registration
	m.RegisterFetcher("mysql", func(db DBExecutor) SchemaFetcher {
		return NewMySQLSchemaFetcher(db)
	})

	// Add ClickHouse schema fetcher registration
	m.RegisterFetcher("clickhouse", func(db DBExecutor) SchemaFetcher {
		return NewClickHouseSchemaFetcher(db)
	})

	m.registerDefaultDrivers()

	return m, nil
}

// RegisterDriver registers a new database driver
func (m *Manager) RegisterDriver(dbType string, driver DatabaseDriver) {
	m.drivers[dbType] = driver
	log.Printf("DBManager -> Registered driver for type: %s", dbType)
}

// RegisterFetcher registers a schema fetcher for a database type
func (m *Manager) RegisterFetcher(dbType string, factory FetcherFactory) {
	m.fetchersMu.Lock()
	defer m.fetchersMu.Unlock()
	m.fetchers[dbType] = factory
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

	// Check if connection already exists
	if existingConn, exists := m.connections[chatID]; exists && existingConn.Status == StatusConnected {
		log.Printf("DBManager -> Connect -> Connection already exists for chatID: %s", chatID)
		return fmt.Errorf("connection already exists for chat ID: %s", chatID)
	}

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

	for id := range existingSubscribers {
		conn.Subscribers[id] = true
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

	conn.OnSchemaChange = func(chatID string) {
		m.doSchemaCheck(chatID)
	}

	log.Printf("DBManager -> Connect -> Connection completed successfully for chatID: %s", chatID)
	return nil
}

// Disconnect closes a database connection
func (m *Manager) Disconnect(chatID, userID string, deleteSchema bool) error {
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

		if deleteSchema {
			log.Printf("DBManager -> Disconnect -> Deleting schema for chatID: %s", chatID)
			// Delete the schema
			m.schemaManager.ClearSchemaCache(chatID)
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
	case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB: // Use same wrapper for both
		return NewPostgresWrapper(conn.DB, m, chatID), nil
	case constants.DatabaseTypeMySQL:
		return NewMySQLWrapper(conn.DB, m, chatID), nil
	case constants.DatabaseTypeClickhouse:
		return NewClickHouseWrapper(conn.DB, m, chatID), nil
	// Add cases for other database types
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Config.Type)
	}
}

// startCleanupRoutine periodically checks for and closes inactive connections
func (m *Manager) startCleanupRoutine() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	log.Printf("DBManager -> startCleanupRoutine -> Starting cleanup routine with interval: %v", cleanupInterval)

	for {
		select {
		case <-m.stopCleanup:
			log.Printf("DBManager -> startCleanupRoutine -> Cleanup routine stopped")
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup closes inactive connections and cleans up stale executions
func (m *Manager) cleanup() {
	start := time.Now()
	connectionsRemoved := 0
	executionsRemoved := 0

	m.mu.Lock()
	for chatID, conn := range m.connections {
		// First check if connection is still active
		if conn.DB != nil {
			sqlDB, err := conn.DB.DB()
			if err != nil || sqlDB.Ping() != nil {
				log.Printf("DBManager -> cleanup -> Connection for chatID %s is no longer active", chatID)
				// Force cleanup of dead connection
				if driver, exists := m.drivers[conn.Config.Type]; exists {
					driver.Disconnect(conn)
				}
				delete(m.connections, chatID)
				connectionsRemoved++
				continue
			}
		}

		if time.Since(conn.LastUsed) > idleTimeout {
			log.Printf("DBManager -> cleanup -> Found idle connection for chatID: %s, last used: %v",
				chatID, conn.LastUsed)

			// Get driver for this connection
			driver, exists := m.drivers[conn.Config.Type]
			if !exists {
				log.Printf("DBManager -> cleanup -> No driver found for type: %s", conn.Config.Type)
				continue
			}

			// Disconnect
			if err := driver.Disconnect(conn); err != nil {
				log.Printf("DBManager -> cleanup -> Error disconnecting: %v", err)
				continue
			}

			// Remove from connections map
			delete(m.connections, chatID)
			log.Printf("DBManager -> cleanup -> Removed inactive connection for chatID: %s", chatID)

			// Remove from Redis
			ctx := context.Background()
			connKey := fmt.Sprintf("conn:%s", chatID)
			if err := m.redisRepo.Del(connKey, ctx); err != nil {
				log.Printf("DBManager -> cleanup -> Failed to remove connection state from cache: %v", err)
			}

			connectionsRemoved++
		}
	}
	m.mu.Unlock()

	// Cleanup stale executions
	m.executionMu.Lock()
	for streamID, execution := range m.activeExecutions {
		// Check if execution has been running for too long (e.g., > 10 minutes)
		if time.Since(execution.StartTime) > 10*time.Minute {
			log.Printf("DBManager -> cleanup -> Found stale execution for streamID: %s, started: %v",
				streamID, execution.StartTime)

			// Cancel the execution
			execution.CancelFunc()

			// Rollback transaction if it exists
			if execution.Tx != nil {
				if err := execution.Tx.Rollback(); err != nil {
					log.Printf("DBManager -> cleanup -> Error rolling back transaction: %v", err)
				}
			}

			delete(m.activeExecutions, streamID)
			log.Printf("DBManager -> cleanup -> Cleaned up stale execution for streamID: %s", streamID)

			executionsRemoved++
		}
	}
	m.executionMu.Unlock()

	// Update metrics
	m.cleanupMetrics = cleanupMetrics{
		lastRun:            start,
		connectionsRemoved: connectionsRemoved,
		executionsRemoved:  executionsRemoved,
	}

	log.Printf("DBManager -> cleanup -> Completed in %v. Removed %d connections and %d executions",
		time.Since(start), connectionsRemoved, executionsRemoved)
}

// Stop gracefully stops the manager and cleans up resources
func (m *Manager) Stop() error {
	log.Printf("DBManager -> Stop -> Stopping manager")

	// Stop cleanup routine
	close(m.stopCleanup)

	// Clean up all active connections
	m.mu.Lock()
	for chatID, conn := range m.connections {
		if driver, exists := m.drivers[conn.Config.Type]; exists {
			if err := driver.Disconnect(conn); err != nil {
				log.Printf("DBManager -> Stop -> Error disconnecting %s: %v", chatID, err)
			}
		}
	}
	m.connections = make(map[string]*Connection)
	m.mu.Unlock()

	// Cancel all active executions
	m.executionMu.Lock()
	for streamID, execution := range m.activeExecutions {
		execution.CancelFunc()
		if execution.Tx != nil {
			if err := execution.Tx.Rollback(); err != nil {
				log.Printf("DBManager -> Stop -> Error rolling back transaction for %s: %v", streamID, err)
			}
		}
	}
	m.activeExecutions = make(map[string]*QueryExecution)
	m.executionMu.Unlock()

	log.Printf("DBManager -> Stop -> Manager stopped successfully")
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

		ticker := time.NewTicker(schemaCheckInterval)

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

	// Get selected collections from the chat service if available
	var selectedTables []string
	if m.streamHandler != nil {
		// Try to get selected collections from the chat service
		selectedCollections, err := m.streamHandler.GetSelectedCollections(chatID)
		if err == nil && selectedCollections != "ALL" && selectedCollections != "" {
			selectedTables = strings.Split(selectedCollections, ",")
			log.Printf("DBManager -> doSchemaCheck -> Using selected collections for chat %s: %v", chatID, selectedTables)
		} else {
			// Default to ALL if there's an error or no specific collections
			selectedTables = []string{"ALL"}
			log.Printf("DBManager -> doSchemaCheck -> Using ALL tables for chat %s", chatID)
		}
	} else {
		// Default to ALL if stream handler is not available
		selectedTables = []string{"ALL"}
	}

	// Force clear any cached schema to ensure we get fresh data
	m.schemaManager.ClearSchemaCache(chatID)

	// Get fresh schema from database
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass selectedTables instead of hardcoded "ALL"
	diff, hasChanged, err := m.schemaManager.CheckSchemaChanges(ctx, chatID, conn, dbConn.Config.Type, selectedTables)
	if err != nil {
		// Check if this is a first-time schema storage error, which we can ignore
		if strings.Contains(err.Error(), "first-time schema storage") || strings.Contains(err.Error(), "key does not exist") {
			log.Printf("DBManager -> doSchemaCheck -> First-time schema storage for chat %s (expected behavior)", chatID)
			return nil
		}

		// Check if this is a schema fetcher not found error, which means we need to register the fetcher
		if strings.Contains(err.Error(), "schema fetcher not found") || strings.Contains(err.Error(), "no schema fetcher registered") {
			log.Printf("DBManager -> doSchemaCheck -> Schema fetcher not found for type %s. This is likely a configuration issue.", dbConn.Config.Type)
			return nil
		}

		return fmt.Errorf("schema check failed: %v", err)
	}

	if diff != nil {
		log.Printf("DBManager -> doSchemaCheck -> Schema diff for chat %s: %+v", chatID, diff)
	}

	if hasChanged {
		log.Printf("DBManager -> doSchemaCheck -> Schema has changed for chat %s: %t", chatID, hasChanged)
		if m.streamHandler != nil {
			m.streamHandler.HandleSchemaChange(dbConn.UserID, chatID, dbConn.StreamID, diff)
		}
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

type QueryExecution struct {
	QueryID     string
	MessageID   string
	StartTime   time.Time
	IsExecuting bool
	IsRollback  bool
	Tx          Transaction // Changed from *sql.Tx to Transaction
	CancelFunc  context.CancelFunc
}

func (m *Manager) CancelQueryExecution(streamID string) {
	m.executionMu.Lock()
	defer m.executionMu.Unlock()

	if execution, exists := m.activeExecutions[streamID]; exists {
		log.Printf("Cancelling query execution for streamID: %s", streamID)

		// Cancel the context first
		execution.CancelFunc()

		// Rollback transaction if it exists
		if execution.Tx != nil {
			if err := execution.Tx.Rollback(); err != nil {
				log.Printf("Error rolling back transaction: %v", err)
			}
		}

		delete(m.activeExecutions, streamID)
		log.Printf("Query execution cancelled for streamID: %s", streamID)
	}
}

// ExecuteQuery executes a query and returns the result, synchronous, no SSE events are sent
func (m *Manager) ExecuteQuery(ctx context.Context, chatID, messageID, queryID, streamID string, query string, queryType string, isRollback bool) (*QueryExecutionResult, *dtos.QueryError) {
	m.executionMu.Lock()

	// Create cancellable context with timeout
	execCtx, cancel := context.WithTimeout(ctx, 1*time.Minute) // 1 minute timeout

	// Track execution
	execution := &QueryExecution{
		QueryID:     queryID,
		MessageID:   messageID,
		StartTime:   time.Now(),
		IsExecuting: true,
		IsRollback:  isRollback,
		CancelFunc:  cancel,
	}
	m.activeExecutions[streamID] = execution
	m.executionMu.Unlock()

	// Ensure cleanup
	defer func() {
		m.executionMu.Lock()
		delete(m.activeExecutions, streamID)
		m.executionMu.Unlock()
		cancel()
	}()

	// Get connection and driver
	conn, exists := m.connections[chatID]
	if !exists {
		return nil, &dtos.QueryError{
			Code:    "NO_CONNECTION_FOUND",
			Message: "no connection found",
			Details: "No connection found for chat ID: " + chatID,
		}
	}

	driver, exists := m.drivers[conn.Config.Type]
	if !exists {
		return nil, &dtos.QueryError{
			Code:    "NO_DRIVER_FOUND",
			Message: "no driver found",
			Details: "No driver found for type: " + conn.Config.Type,
		}
	}

	log.Printf("Manager -> ExecuteQuery -> Driver: %v", driver)
	// Begin transaction
	tx := driver.BeginTx(execCtx, conn)
	if tx == nil {
		return nil, &dtos.QueryError{
			Code:    "FAILED_TO_START_TRANSACTION",
			Message: "failed to start transaction",
			Details: "Failed to start transaction",
		}
	}
	execution.Tx = tx

	// Execute query with proper cancellation handling
	var result *QueryExecutionResult
	done := make(chan struct{})
	var queryErr *dtos.QueryError

	go func() {
		defer close(done)
		log.Printf("Manager -> ExecuteQuery -> Executing query: %v", query)
		result = tx.ExecuteQuery(execCtx, conn, query, queryType)
		log.Printf("Manager -> ExecuteQuery -> Result: %v", result)
		if result.Error != nil {
			queryErr = result.Error
		}
	}()

	select {
	case <-execCtx.Done():
		if err := tx.Rollback(); err != nil {
			log.Printf("Error rolling back transaction: %v", err)
		}
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, &dtos.QueryError{
				Code:    "QUERY_EXECUTION_TIMED_OUT",
				Message: "query execution timed out",
				Details: "Query execution timed out",
			}
		}
		return nil, &dtos.QueryError{
			Code:    "QUERY_EXECUTION_CANCELLED",
			Message: "query execution cancelled",
			Details: "Query execution cancelled",
		}

	case <-done:
		if queryErr != nil {
			if err := tx.Rollback(); err != nil {
				log.Printf("Error rolling back transaction: %v", err)
			}
			return result, queryErr
		}
		if err := tx.Commit(); err != nil {
			return nil, &dtos.QueryError{
				Code:    "QUERY_EXECUTION_FAILED",
				Message: "query execution failed",
				Details: err.Error(),
			}
		}
		log.Println("Manager -> ExecuteQuery -> Commit completed:")
		log.Printf("Manager -> ExecuteQuery -> Query type: %v", queryType)

		go func() {
			log.Println("Manager -> ExecuteQuery -> Triggering schema check")
			time.Sleep(2 * time.Second)
			switch conn.Config.Type {
			case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
				if queryType == "DDL" || queryType == "ALTER" || queryType == "DROP" {
					if conn.OnSchemaChange != nil {
						conn.OnSchemaChange(conn.ChatID)
					}
				}
			case constants.DatabaseTypeMySQL:
				if queryType == "DDL" || queryType == "ALTER" || queryType == "DROP" {
					if conn.OnSchemaChange != nil {
						conn.OnSchemaChange(conn.ChatID)
					}
				}
			case constants.DatabaseTypeClickhouse:
				if queryType == "DDL" || queryType == "ALTER" || queryType == "DROP" {
					if conn.OnSchemaChange != nil {
						conn.OnSchemaChange(conn.ChatID)
					}
				}
			}
		}()

		return result, nil
	}
}

// TestConnection tests if the provided credentials are valid without creating a persistent connection
func (m *Manager) TestConnection(config *ConnectionConfig) error {
	switch config.Type {
	case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
		dsn := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			config.Host, config.Port, *config.Username, *config.Password, config.Database,
		)
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return fmt.Errorf("failed to create connection: %v", err)
		}
		defer db.Close()

		err = db.Ping()
		if err != nil {
			return fmt.Errorf("failed to conne: %v", err)
		}

	case constants.DatabaseTypeMySQL:
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s",
			*config.Username, *config.Password, config.Host, config.Port, config.Database,
		)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return fmt.Errorf("failed to create MySQL connection: %v", err)
		}
		defer db.Close()

		err = db.Ping()
		if err != nil {
			return fmt.Errorf("failed to connect to MySQL: %v", err)
		}

	case constants.DatabaseTypeClickhouse:
		dsn := fmt.Sprintf(
			"clickhouse://%s:%s@%s:%s/%s",
			*config.Username, *config.Password, config.Host, config.Port, config.Database,
		)
		db, err := sql.Open("clickhouse", dsn)
		if err != nil {
			return fmt.Errorf("failed to create ClickHouse connection: %v", err)
		}
		defer db.Close()

		err = db.Ping()
		if err != nil {
			return fmt.Errorf("failed to connect to ClickHouse: %v", err)
		}

	case constants.DatabaseTypeMongoDB:
		uri := fmt.Sprintf(
			"mongodb://%s:%s@%s:%s/%s",
			*config.Username, *config.Password, config.Host, config.Port, config.Database,
		)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			return fmt.Errorf("failed to create MongoDB connection: %v", err)
		}
		defer client.Disconnect(ctx)

		err = client.Ping(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to connect to MongoDB: %v", err)
		}

	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}

	return nil
}

// FormatSchemaWithExamples formats the schema with example records for LLM
func (m *Manager) FormatSchemaWithExamples(ctx context.Context, chatID string, selectedCollections []string) (string, error) {
	log.Printf("DBManager -> FormatSchemaWithExamples -> Starting for chatID: %s with selected collections: %v", chatID, selectedCollections)

	// Get connection with read lock to ensure thread safety
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("DBManager -> FormatSchemaWithExamples -> Connection not found for chatID: %s", chatID)
		return "", fmt.Errorf("connection not found for chat ID: %s", chatID)
	}

	// Get database executor
	db, err := m.GetConnection(chatID)
	if err != nil {
		log.Printf("DBManager -> FormatSchemaWithExamples -> Error getting executor: %v", err)
		return "", fmt.Errorf("failed to get database executor: %v", err)
	}

	// Use schema manager to format schema with examples and selected collections
	formattedSchema, err := m.schemaManager.FormatSchemaWithExamplesAndCollections(ctx, chatID, db, conn.Config.Type, selectedCollections)
	if err != nil {
		log.Printf("DBManager -> FormatSchemaWithExamples -> Error formatting schema: %v", err)
		return "", fmt.Errorf("failed to format schema with examples: %v", err)
	}

	log.Printf("DBManager -> FormatSchemaWithExamples -> Successfully formatted schema for chatID: %s", chatID)
	return formattedSchema, nil
}

// GetSchemaWithExamples gets the schema with example records
func (m *Manager) GetSchemaWithExamples(ctx context.Context, chatID string, selectedCollections []string) (*SchemaStorage, error) {
	log.Printf("DBManager -> GetSchemaWithExamples -> Starting for chatID: %s with selected collections: %v", chatID, selectedCollections)

	// Get connection with read lock to ensure thread safety
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("DBManager -> GetSchemaWithExamples -> Connection not found for chatID: %s", chatID)
		return nil, fmt.Errorf("connection not found for chat ID: %s", chatID)
	}

	// Get database executor
	db, err := m.GetConnection(chatID)
	if err != nil {
		log.Printf("DBManager -> GetSchemaWithExamples -> Error getting executor: %v", err)
		return nil, fmt.Errorf("failed to get database executor: %v", err)
	}

	// Use schema manager to get schema with examples
	storage, err := m.schemaManager.GetSchemaWithExamples(ctx, chatID, db, conn.Config.Type, selectedCollections)
	if err != nil {
		log.Printf("DBManager -> GetSchemaWithExamples -> Error getting schema: %v", err)
		return nil, fmt.Errorf("failed to get schema with examples: %v", err)
	}

	log.Printf("DBManager -> GetSchemaWithExamples -> Successfully retrieved schema for chatID: %s", chatID)
	return storage, nil
}

// RefreshSchemaWithExamples refreshes the schema and returns it with example records
func (m *Manager) RefreshSchemaWithExamples(ctx context.Context, chatID string, selectedCollections []string) (string, error) {
	log.Printf("DBManager -> RefreshSchemaWithExamples -> Starting for chatID: %s with selected collections: %v", chatID, selectedCollections)

	// Create a new context with a longer timeout specifically for this operation
	schemaCtx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	// Get connection with read lock to ensure thread safety
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Connection not found for chatID: %s", chatID)
		return "", fmt.Errorf("connection not found for chat ID: %s", chatID)
	}

	// Get database executor
	db, err := m.GetConnection(chatID)
	if err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Error getting executor: %v", err)
		return "", fmt.Errorf("failed to get database executor: %v", err)
	}

	// Clear schema cache to force refresh
	m.schemaManager.ClearSchemaCache(chatID)
	log.Printf("DBManager -> RefreshSchemaWithExamples -> Cleared schema cache for chatID: %s", chatID)

	// Check for context cancellation
	if err := schemaCtx.Err(); err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Context cancelled: %v", err)
		return "", fmt.Errorf("operation cancelled: %v", err)
	}

	// Force a fresh schema fetch by directly calling GetSchema first
	log.Printf("DBManager -> RefreshSchemaWithExamples -> Forcing fresh schema fetch for chatID: %s", chatID)

	// Convert selectedCollections to the format expected by GetSchema
	var selectedTables []string
	if len(selectedCollections) == 0 || (len(selectedCollections) == 1 && selectedCollections[0] == "ALL") {
		selectedTables = []string{"ALL"}
	} else {
		selectedTables = selectedCollections
	}

	// Fetch fresh schema directly with the longer timeout context
	freshSchema, err := m.schemaManager.GetSchema(schemaCtx, chatID, db, conn.Config.Type, selectedTables)
	if err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Error fetching fresh schema: %v", err)
		return "", fmt.Errorf("failed to fetch fresh schema: %v", err)
	}

	// Store the fresh schema
	err = m.schemaManager.storeSchema(schemaCtx, chatID, freshSchema, db, conn.Config.Type)
	if err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Error storing fresh schema: %v", err)
		// Continue anyway, as we have the fresh schema
	}

	// Check for context cancellation
	if err := schemaCtx.Err(); err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Context cancelled after schema fetch: %v", err)
		return "", fmt.Errorf("operation cancelled: %v", err)
	}

	// Format schema with examples and selected collections
	formattedSchema, err := m.schemaManager.FormatSchemaWithExamplesAndCollections(schemaCtx, chatID, db, conn.Config.Type, selectedCollections)
	if err != nil {
		log.Printf("DBManager -> RefreshSchemaWithExamples -> Error formatting schema: %v", err)
		return "", fmt.Errorf("failed to format schema with examples: %v", err)
	}

	log.Printf("DBManager -> RefreshSchemaWithExamples -> Successfully refreshed schema for chatID: %s (schema length: %d)", chatID, len(formattedSchema))
	return formattedSchema, nil
}

func (m *Manager) registerDefaultDrivers() {
	// Register PostgreSQL driver
	m.RegisterDriver("postgresql", NewPostgresDriver())

	// Register YugabyteDB driver (uses PostgreSQL driver)
	m.RegisterDriver("yugabytedb", NewPostgresDriver())

	// Register MySQL driver
	m.RegisterDriver("mysql", NewMySQLDriver())

	// Register ClickHouse driver
	m.RegisterDriver("clickhouse", NewClickHouseDriver())
}
