package dbmanager

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	// Database drivers
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	mysqldriver "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq" // PostgreSQL/YugabyteDB Driver
	"gorm.io/gorm"

	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/pkg/redis"

	"crypto/tls"
	"crypto/x509"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
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

// DatabasePool represents a shared database connection with reference counting
type DatabasePool struct {
	DB         *sql.DB
	GORMDB     *gorm.DB
	RefCount   int
	Config     ConnectionConfig
	LastUsed   time.Time
	Mutex      sync.Mutex // For thread-safe reference counting
	MongoDBObj interface{}
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
	dbPools          map[string]*DatabasePool // key: hash of connection config
	dbPoolsMu        sync.RWMutex
	poolMetrics      struct {
		totalPools       int
		totalConnections int
		reuseCount       int
	}
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
		dbPools:          make(map[string]*DatabasePool),
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

	m.RegisterFetcher("mongodb", func(db DBExecutor) SchemaFetcher {
		return NewMongoDBSchemaFetcher(db)
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

	// Generate a unique key for this database configuration
	configKey := generateConfigKey(config)
	log.Printf("DBManager -> Connect -> Generated config key: %s", configKey)

	// Check if we already have a connection to this database
	var conn *Connection
	var err error

	m.dbPoolsMu.RLock()
	pool, poolExists := m.dbPools[configKey]
	m.dbPoolsMu.RUnlock()

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

	if poolExists {
		// Use existing connection from pool
		pool.Mutex.Lock()
		pool.RefCount++
		pool.LastUsed = time.Now()
		pool.Mutex.Unlock()

		log.Printf("DBManager -> Connect -> Reusing existing connection from pool, refCount: %d", pool.RefCount)

		// Create a new connection using the shared pool
		conn = &Connection{
			DB:          pool.GORMDB,
			LastUsed:    time.Now(),
			Status:      StatusConnected,
			Config:      config,
			UserID:      userID,
			ChatID:      chatID,
			StreamID:    streamID,
			Subscribers: make(map[string]bool),
			SubLock:     sync.RWMutex{},
			ConfigKey:   configKey, // Store the config key for reference
		}

		// Set MongoDBObj for MongoDB connections when reusing from pool
		if config.Type == "mongodb" && pool.MongoDBObj != nil {
			conn.MongoDBObj = pool.MongoDBObj
			log.Printf("DBManager -> Connect -> Set MongoDBObj from pool for MongoDB connection")
		}

		// Update metrics
		m.poolMetrics.reuseCount++
	} else {
		// Create a new connection
		conn, err = driver.Connect(config)
		if err != nil {
			log.Printf("DBManager -> Connect -> Driver connection failed: %v", err)
			return err
		}

		log.Printf("DBManager -> Connect -> Driver connection successful, creating new pool")

		// Create and store the new pool
		newPool := &DatabasePool{
			DB:       nil, // The driver doesn't expose sql.DB directly
			GORMDB:   conn.DB,
			RefCount: 1,
			Config:   config,
			LastUsed: time.Now(),
		}

		// For MongoDB, store the MongoDB client in the pool
		if config.Type == "mongodb" {
			newPool.MongoDBObj = conn.MongoDBObj
		}

		m.dbPoolsMu.Lock()
		m.dbPools[configKey] = newPool
		m.dbPoolsMu.Unlock()

		// Update metrics
		m.poolMetrics.totalPools++

		// Initialize connection fields
		conn.LastUsed = time.Now()
		conn.Status = StatusConnected
		conn.Config = config
		conn.UserID = userID
		conn.ChatID = chatID
		conn.StreamID = streamID
		conn.ConfigKey = configKey
	}

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

	return nil
}

// Disconnect closes a database connection
func (m *Manager) Disconnect(chatID, userID string, deleteSchema bool) error {
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("connection not found for chat %s", chatID)
	}

	log.Printf("DBManager -> Disconnect -> Starting disconnect for chatID: %s", chatID)

	// Get the config key for the shared pool
	configKey := conn.ConfigKey

	// Decrement reference count in the pool
	m.dbPoolsMu.Lock()
	pool, poolExists := m.dbPools[configKey]

	if poolExists {
		pool.Mutex.Lock()
		pool.RefCount--
		refCount := pool.RefCount
		pool.Mutex.Unlock()

		log.Printf("DBManager -> Disconnect -> Decremented pool refCount to %d", refCount)

		// If reference count is zero, close the actual connection
		if refCount <= 0 {
			// Get the driver for this database type
			driver := m.drivers[conn.Config.Type]
			if driver != nil {
				if err := driver.Disconnect(conn); err != nil {
					m.dbPoolsMu.Unlock()
					return fmt.Errorf("failed to disconnect: %v", err)
				}
			}

			// Remove from pool
			delete(m.dbPools, configKey)
			log.Printf("DBManager -> Disconnect -> Removed pool from dbPools map")
		}
	}
	m.dbPoolsMu.Unlock()

	// Remove from connections map
	m.mu.Lock()
	delete(m.connections, chatID)
	m.mu.Unlock()

	log.Printf("DBManager -> Disconnect -> Removed connection from connections map")

	// Delete schema if requested
	if deleteSchema && m.schemaManager != nil {
		m.schemaManager.ClearSchemaCache(chatID)
		log.Printf("DBManager -> Disconnect -> Cleared schema cache for chatID: %s", chatID)
	}

	// Notify subscribers
	m.notifySubscribers(chatID, userID, StatusDisconnected, "")
	log.Printf("DBManager -> Disconnect -> Notified subscribers")

	return nil
}

// GetConnection returns a database connection for a chat
func (m *Manager) GetConnection(chatID string) (DBExecutor, error) {
	m.mu.RLock()
	conn, exists := m.connections[chatID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("connection not found for chat %s", chatID)
	}

	// Update LastUsed in both connection and pool
	conn.LastUsed = time.Now()

	// Also update the pool's LastUsed
	if conn.ConfigKey != "" {
		m.dbPoolsMu.RLock()
		if pool, exists := m.dbPools[conn.ConfigKey]; exists {
			pool.Mutex.Lock()
			pool.LastUsed = time.Now()
			pool.Mutex.Unlock()
		}
		m.dbPoolsMu.RUnlock()
	}

	// Create appropriate wrapper based on database type
	switch conn.Config.Type {
	case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
		return NewPostgresWrapper(conn.DB, m, chatID), nil
	case constants.DatabaseTypeMySQL:
		return NewMySQLWrapper(conn.DB, m, chatID), nil
	case constants.DatabaseTypeClickhouse:
		return NewClickHouseWrapper(conn.DB, m, chatID), nil
	case constants.DatabaseTypeMongoDB:
		// For MongoDB, we use the MongoDBObj field instead of DB
		_, ok := conn.MongoDBObj.(*MongoDBWrapper)
		if !ok {
			return nil, fmt.Errorf("invalid MongoDB connection")
		}
		executor, err := NewMongoDBExecutor(conn)
		if err != nil {
			return nil, fmt.Errorf("failed to create MongoDB executor: %v", err)
		}
		return executor, nil
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

// cleanup removes idle connections
func (m *Manager) cleanup() {
	now := time.Now()
	m.cleanupMetrics.lastRun = now

	// Cleanup connections
	m.mu.Lock()
	for chatID, conn := range m.connections {
		if time.Since(conn.LastUsed) > idleTimeout {
			log.Printf("DBManager -> cleanup -> Removing idle connection for chatID: %s (idle for %v)", chatID, time.Since(conn.LastUsed))

			// Don't actually disconnect here, just remove from the map
			delete(m.connections, chatID)
			m.cleanupMetrics.connectionsRemoved++
		}
	}
	m.mu.Unlock()

	// Cleanup database pools
	m.dbPoolsMu.Lock()
	for key, pool := range m.dbPools {
		pool.Mutex.Lock()
		if pool.RefCount <= 0 && time.Since(pool.LastUsed) > idleTimeout {
			log.Printf("DBManager -> cleanup -> Removing idle connection pool: %s (idle for %v)", key, time.Since(pool.LastUsed))

			// Close the connection
			if pool.GORMDB != nil {
				sqlDB, err := pool.GORMDB.DB()
				if err == nil && sqlDB != nil {
					sqlDB.Close()
				}
			}
			delete(m.dbPools, key)
		}
		pool.Mutex.Unlock()
	}
	m.dbPoolsMu.Unlock()

	// Cleanup active executions
	m.executionMu.Lock()
	for streamID, execution := range m.activeExecutions {
		if !execution.IsExecuting && time.Since(execution.StartTime) > idleTimeout {
			log.Printf("DBManager -> cleanup -> Removing idle execution for streamID: %s (idle for %v)", streamID, time.Since(execution.StartTime))
			delete(m.activeExecutions, streamID)
			m.cleanupMetrics.executionsRemoved++
		}
	}
	m.executionMu.Unlock()
}

// Stop closes all connections and stops the cleanup routine
func (m *Manager) Stop() error {
	log.Println("DBManager -> Stop -> Stopping manager")

	// Signal cleanup routine to stop
	close(m.stopCleanup)
	log.Println("DBManager -> Stop -> Signaled cleanup routine to stop")

	// Close all connections
	m.mu.Lock()
	for chatID, conn := range m.connections {
		if driver, exists := m.drivers[conn.Config.Type]; exists {
			if err := driver.Disconnect(conn); err != nil {
				log.Printf("DBManager -> Stop -> Error disconnecting chat %s: %v", chatID, err)
			} else {
				log.Printf("DBManager -> Stop -> Disconnected chat %s", chatID)
			}
		}
	}
	m.connections = make(map[string]*Connection)
	m.mu.Unlock()
	log.Println("DBManager -> Stop -> Closed all connections")

	// Close all database pools
	m.dbPoolsMu.Lock()
	for key, pool := range m.dbPools {
		if pool.GORMDB != nil {
			sqlDB, err := pool.GORMDB.DB()
			if err == nil && sqlDB != nil {
				sqlDB.Close()
				log.Printf("DBManager -> Stop -> Closed pool: %s", key)
			}
		}
		delete(m.dbPools, key)
	}
	m.dbPoolsMu.Unlock()
	log.Println("DBManager -> Stop -> Closed all connection pools")

	// Cancel any active executions
	m.executionMu.Lock()
	for streamID, execution := range m.activeExecutions {
		execution.CancelFunc()
		if execution.Tx != nil {
			if err := execution.Tx.Rollback(); err != nil {
				log.Printf("DBManager -> Stop -> Error rolling back transaction for stream %s: %v", streamID, err)
			}
		}
		log.Printf("DBManager -> Stop -> Cancelled execution for stream %s", streamID)
	}
	m.activeExecutions = make(map[string]*QueryExecution)
	m.executionMu.Unlock()
	log.Println("DBManager -> Stop -> Cancelled all active executions")

	log.Println("DBManager -> Stop -> Manager stopped successfully")
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

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		// Call Schema change only when schema is empty
		if _, err := m.schemaManager.getStoredSchema(ctx, chatID); err != nil {
			log.Printf("DBManager -> StartSchemaTracking -> err: %v", err)
			// Do initial schema check
			if err := m.doSchemaCheck(chatID); err != nil {
				log.Printf("DBManager -> StartSchemaTracking -> Initial schema check failed: %v", err)
			}
		}

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
				defer cancel()

				// Call Schema change only when schema is empty
				if _, err := m.schemaManager.getStoredSchema(ctx, chatID); err != nil {
					log.Printf("DBManager -> StartSchemaTracking -> err: %v", err)
					// Do initial schema check
					if err := m.doSchemaCheck(chatID); err != nil {
						log.Printf("DBManager -> StartSchemaTracking -> Initial schema check failed: %v", err)
					}
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
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

	// For MongoDB connections
	if conn.Config.Type == "mongodb" {
		if wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper); ok && wrapper != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err := wrapper.Client.Ping(ctx, nil)
			return err == nil
		}
		return false
	}

	// For SQL connections
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

// ExecuteQuery executes a query and returns the result, synchronous, no SSE events are sent, findCount is used to strictly get the number/count of records that the query returns
func (m *Manager) ExecuteQuery(ctx context.Context, chatID, messageID, queryID, streamID string, query string, queryType string, isRollback bool, findCount bool) (*QueryExecutionResult, *dtos.QueryError) {
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

	// Check if transaction has an error (MongoDB transaction might return a non-nil transaction with an error)
	if mongoTx, ok := tx.(*MongoDBTransaction); ok && mongoTx.Error != nil {
		log.Printf("Manager -> ExecuteQuery -> MongoDB transaction error: %v", mongoTx.Error)
		return nil, &dtos.QueryError{
			Code:    "FAILED_TO_START_TRANSACTION",
			Message: "failed to start transaction",
			Details: mongoTx.Error.Error(),
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
		result = tx.ExecuteQuery(execCtx, conn, query, queryType, findCount)
		// log.Printf("Manager -> ExecuteQuery -> Result: %v", result)
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
			case constants.DatabaseTypeMongoDB:
				if queryType == "CREATE_COLLECTION" || queryType == "DROP_COLLECTION" {
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
	var tempFiles []string

	switch config.Type {
	case constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB:
		var dsn string
		port := "5432" // Default port
		if config.Type == constants.DatabaseTypeYugabyteDB {
			port = "5433" // Default port
		}

		if config.Port != nil && *config.Port != "" {
			port = *config.Port
		}
		// Base connection parameters
		baseParams := fmt.Sprintf(
			"host=%s port=%s user=%s dbname=%s",
			config.Host, port, *config.Username, config.Database,
		)

		// Add password if provided
		if config.Password != nil {
			baseParams += fmt.Sprintf(" password=%s", *config.Password)
		}

		// Configure SSL/TLS
		if config.UseSSL {
			// Always use verify-full mode for maximum security
			baseParams += " sslmode=verify-full"

			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := prepareCertificatesFromURLs(*config)
			if err != nil {
				return err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Add certificate paths to connection string
			if certPath != "" {
				baseParams += fmt.Sprintf(" sslcert=%s", certPath)
			}

			if keyPath != "" {
				baseParams += fmt.Sprintf(" sslkey=%s", keyPath)
			}

			if rootCertPath != "" {
				baseParams += fmt.Sprintf(" sslrootcert=%s", rootCertPath)
			}
		} else {
			baseParams += " sslmode=disable"
		}

		dsn = baseParams

		// Open connection
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			// Clean up temporary files
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return fmt.Errorf("failed to create connection: %v", err)
		}

		// Test connection
		err = db.Ping()

		// Close connection
		db.Close()

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}

		if err != nil {
			return err
		}

		return nil

	case constants.DatabaseTypeMySQL:
		var dsn string
		port := "3306" // Default port for MySQL

		if config.Port != nil && *config.Port != "" {
			port = *config.Port
		}

		// Base connection parameters
		if config.Password != nil {
			dsn = fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s",
				*config.Username, *config.Password, config.Host, port, config.Database,
			)
		} else {
			dsn = fmt.Sprintf(
				"%s@tcp(%s:%s)/%s",
				*config.Username, config.Host, port, config.Database,
			)
		}

		// Add parameters
		dsn += "?parseTime=true"

		// Configure SSL/TLS
		if config.UseSSL {
			// Create a unique TLS config name
			tlsConfigName := fmt.Sprintf("custom-test-%d", time.Now().UnixNano())

			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := prepareCertificatesFromURLs(*config)
			if err != nil {
				return err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Create TLS config
			tlsConfig := &tls.Config{
				ServerName: config.Host,
				MinVersion: tls.VersionTLS12,
			}

			// Add client certificates if provided
			if certPath != "" && keyPath != "" {
				cert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to load client certificates: %v", err)
				}
				tlsConfig.Certificates = []tls.Certificate{cert}
			}

			// Add CA certificate if provided
			if rootCertPath != "" {
				rootCertPool := x509.NewCertPool()
				pem, err := ioutil.ReadFile(rootCertPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to read CA certificate: %v", err)
				}
				if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to append CA certificate")
				}
				tlsConfig.RootCAs = rootCertPool
			}

			// Register TLS config
			mysqldriver.RegisterTLSConfig(tlsConfigName, tlsConfig)

			// Add TLS config to DSN
			dsn += "&tls=" + tlsConfigName
		}

		// Open connection
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			// Clean up temporary files
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return fmt.Errorf("failed to create connection: %v", err)
		}

		// Test connection
		err = db.Ping()

		// Close connection
		db.Close()

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}

		if err != nil {
			return fmt.Errorf("failed to connect to database: %v", err)
		}

		return nil

	case constants.DatabaseTypeClickhouse:
		var dsn string
		port := "9000" // Default port for ClickHouse

		if config.Port != nil && *config.Port != "" {
			port = *config.Port
		}

		// Base connection parameters
		protocol := "tcp"

		// Configure SSL/TLS
		if config.UseSSL {
			// Fetch certificates from URLs
			_, _, _, certTempFiles, err := prepareCertificatesFromURLs(*config)
			if err != nil {
				return err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Use secure protocol
			protocol = "https"
		}

		// Build DSN
		if config.Password != nil {
			dsn = fmt.Sprintf("%s://%s:%s@%s:%s/%s",
				protocol, *config.Username, *config.Password, config.Host, port, config.Database)
		} else {
			dsn = fmt.Sprintf("%s://%s@%s:%s/%s",
				protocol, *config.Username, config.Host, port, config.Database)
		}

		// Add parameters
		dsn += "?dial_timeout=10s&read_timeout=20s"

		// Add secure parameter if using SSL
		if config.UseSSL {
			dsn += "&secure=true"
		}

		// Open connection
		db, err := sql.Open("clickhouse", dsn)
		if err != nil {
			// Clean up temporary files
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return fmt.Errorf("failed to create connection: %v", err)
		}

		// Test connection
		err = db.Ping()

		// Close connection
		db.Close()

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}

		if err != nil {
			return fmt.Errorf("failed to connect to database: %v", err)
		}

		return nil

	case constants.DatabaseTypeMongoDB:
		log.Printf("DBManager -> TestConnection -> Testing MongoDB connection at %s:%s", config.Host, config.Port)

		var uri string
		port := "27017" // Default port for MongoDB

		// Check if we're using SRV records (mongodb+srv://)
		isSRV := strings.Contains(config.Host, ".mongodb.net")
		protocol := "mongodb"
		if isSRV {
			protocol = "mongodb+srv"
		}

		// Validate port value if not using SRV
		if !isSRV && config.Port != nil {
			// Log the port value for debugging
			log.Printf("DBManager -> TestConnection -> Port value before validation: %v", *config.Port)

			// Check if port is empty
			if *config.Port == "" {
				log.Printf("DBManager -> TestConnection -> Port is empty, using default port 27017")
			} else {
				port = *config.Port

				// Only validate port as numeric if it doesn't contain base64 characters
				// (which would indicate it's encrypted)
				if !strings.Contains(port, "+") && !strings.Contains(port, "/") && !strings.Contains(port, "=") {
					// Verify port is numeric
					if _, err := strconv.Atoi(port); err != nil {
						log.Printf("DBManager -> TestConnection -> Non-numeric port value: %v, might be encrypted", port)
						// Don't return error for potentially encrypted ports
					}
				}
			}
		}

		// Base connection parameters with authentication
		if config.Username != nil && *config.Username != "" {
			// URL encode username and password to handle special characters
			encodedUsername := url.QueryEscape(*config.Username)
			encodedPassword := url.QueryEscape(*config.Password)

			if isSRV {
				// For SRV records, don't include port
				uri = fmt.Sprintf("%s://%s:%s@%s/%s",
					protocol, encodedUsername, encodedPassword, config.Host, config.Database)
			} else {
				// Include port for standard connections
				uri = fmt.Sprintf("%s://%s:%s@%s:%s/%s",
					protocol, encodedUsername, encodedPassword, config.Host, port, config.Database)
			}
		} else {
			// Without authentication
			if isSRV {
				// For SRV records, don't include port
				uri = fmt.Sprintf("%s://%s/%s", protocol, config.Host, config.Database)
			} else {
				// Include port for standard connections
				uri = fmt.Sprintf("%s://%s:%s/%s", protocol, config.Host, port, config.Database)
			}
		}

		// Log the final URI (with sensitive parts masked)
		maskedUri := uri
		if config.Password != nil && *config.Password != "" {
			maskedUri = strings.Replace(maskedUri, *config.Password, "********", -1)
		}
		log.Printf("DBManager -> TestConnection -> Connection URI: %s", maskedUri)

		// Add connection options
		if isSRV {
			uri += "?retryWrites=true&w=majority"
		}

		// Configure client options
		clientOptions := options.Client().ApplyURI(uri)

		// Configure SSL/TLS
		if config.UseSSL {
			// Fetch certificates from URLs
			certPath, keyPath, rootCertPath, certTempFiles, err := prepareCertificatesFromURLs(*config)
			if err != nil {
				return err
			}

			// Track temporary files for cleanup
			tempFiles = certTempFiles

			// Configure TLS
			tlsConfig := &tls.Config{
				InsecureSkipVerify: false, // Always verify certificates
			}

			// Add client certificates if provided
			if certPath != "" && keyPath != "" {
				cert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to load client certificates: %v", err)
				}
				tlsConfig.Certificates = []tls.Certificate{cert}
			}

			// Add root CA if provided
			if rootCertPath != "" {
				rootCA, err := os.ReadFile(rootCertPath)
				if err != nil {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to read root CA: %v", err)
				}

				rootCertPool := x509.NewCertPool()
				if ok := rootCertPool.AppendCertsFromPEM(rootCA); !ok {
					// Clean up temporary files
					for _, file := range tempFiles {
						os.Remove(file)
					}
					return fmt.Errorf("failed to parse root CA certificate")
				}

				tlsConfig.RootCAs = rootCertPool
			}

			clientOptions.SetTLSConfig(tlsConfig)
		}

		// Connect to MongoDB with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			// Clean up temporary files
			for _, file := range tempFiles {
				os.Remove(file)
			}
			log.Printf("DBManager -> TestConnection -> Error connecting to MongoDB: %v", err)
			return fmt.Errorf("failed to connect to MongoDB: %v", err)
		}

		// Ping the database to verify connection
		err = client.Ping(ctx, readpref.Primary())

		// Disconnect regardless of ping result
		client.Disconnect(ctx)

		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}

		if err != nil {
			log.Printf("DBManager -> TestConnection -> Error pinging MongoDB: %v", err)
			return fmt.Errorf("failed to ping MongoDB: %v", err)
		}

		log.Printf("DBManager -> TestConnection -> Successfully connected to MongoDB")
		return nil

	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}
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

	// Register MongoDB driver
	m.RegisterDriver("mongodb", NewMongoDBDriver())

	// Register MongoDB schema fetcher
	m.RegisterFetcher("mongodb", func(db DBExecutor) SchemaFetcher {
		return NewMongoDBSchemaFetcher(db)
	})
}

// generateConfigKey creates a unique string key for a database configuration
func generateConfigKey(config ConnectionConfig) string {
	var username string
	if config.Username != nil {
		username = *config.Username
	}

	port := ""
	if config.Port != nil {
		port = *config.Port
	}
	// Create a unique key based on connection details
	key := fmt.Sprintf("%s:%s:%s:%s:%s",
		config.Type,
		config.Host,
		port,
		username,
		config.Database)

	return key
}

// GetPoolMetrics returns metrics about the connection pools
func (m *Manager) GetPoolMetrics() map[string]interface{} {
	m.dbPoolsMu.RLock()
	defer m.dbPoolsMu.RUnlock()

	totalRefs := 0
	for _, pool := range m.dbPools {
		pool.Mutex.Lock()
		totalRefs += pool.RefCount
		pool.Mutex.Unlock()
	}

	return map[string]interface{}{
		"total_pools":       len(m.dbPools),
		"total_connections": totalRefs,
		"reuse_count":       m.poolMetrics.reuseCount,
	}
}

// fetchCertificateFromURL downloads a certificate from a URL and stores it temporarily
func fetchCertificateFromURL(url string) (string, error) {
	// Create a temporary file
	tmpFile, err := ioutil.TempFile("", "cert-*.pem")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer tmpFile.Close()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Fetch the certificate
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch certificate from URL: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch certificate, status: %s", resp.Status)
	}

	// Copy the certificate to the temporary file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save certificate: %v", err)
	}

	// Return the path to the temporary file
	return tmpFile.Name(), nil
}

// prepareCertificatesFromURLs fetches certificates from URLs and returns their local paths
func prepareCertificatesFromURLs(config ConnectionConfig) (certPath, keyPath, rootCertPath string, tempFiles []string, err error) {
	// Fetch client certificate if URL provided
	if config.SSLCertURL != nil && *config.SSLCertURL != "" {
		certPath, err = fetchCertificateFromURL(*config.SSLCertURL)
		if err != nil {
			// Clean up any files already created
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return "", "", "", nil, fmt.Errorf("failed to fetch client certificate: %v", err)
		}
		tempFiles = append(tempFiles, certPath)
	}

	// Fetch client key if URL provided
	if config.SSLKeyURL != nil && *config.SSLKeyURL != "" {
		keyPath, err = fetchCertificateFromURL(*config.SSLKeyURL)
		if err != nil {
			// Clean up any files already created
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return "", "", "", nil, fmt.Errorf("failed to fetch client key: %v", err)
		}
		tempFiles = append(tempFiles, keyPath)
	}

	// Fetch CA certificate if URL provided
	if config.SSLRootCertURL != nil && *config.SSLRootCertURL != "" {
		rootCertPath, err = fetchCertificateFromURL(*config.SSLRootCertURL)
		if err != nil {
			// Clean up any files already created
			for _, file := range tempFiles {
				os.Remove(file)
			}
			return "", "", "", nil, fmt.Errorf("failed to fetch CA certificate: %v", err)
		}
		tempFiles = append(tempFiles, rootCertPath)
	}

	return certPath, keyPath, rootCertPath, tempFiles, nil
}
