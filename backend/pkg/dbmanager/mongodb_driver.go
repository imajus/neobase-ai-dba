package dbmanager

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"crypto/x509"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoDBSchema represents the schema of a MongoDB database
type MongoDBSchema struct {
	Collections map[string]MongoDBCollection
	Indexes     map[string][]MongoDBIndex
	Version     int64
	UpdatedAt   time.Time
}

// MongoDBCollection represents a MongoDB collection
type MongoDBCollection struct {
	Name           string
	Fields         map[string]MongoDBField
	Indexes        []MongoDBIndex
	DocumentCount  int64
	SampleDocument bson.M
}

// MongoDBField represents a field in a MongoDB collection
type MongoDBField struct {
	Name         string
	Type         string
	IsRequired   bool
	IsArray      bool
	NestedFields map[string]MongoDBField
	Frequency    float64 // Percentage of documents containing this field
}

// MongoDBIndex represents an index in a MongoDB collection
type MongoDBIndex struct {
	Name     string
	Keys     bson.D
	IsUnique bool
	IsSparse bool
}

// GetSchema retrieves the schema information for MongoDB
func (d *MongoDBDriver) GetSchema(ctx context.Context, db DBExecutor, selectedCollections []string) (*SchemaInfo, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MongoDBDriver -> GetSchema -> Context cancelled: %v", err)
		return nil, err
	}

	// Get the MongoDB wrapper
	executor, ok := db.(*MongoDBExecutor)
	if !ok {
		return nil, fmt.Errorf("invalid MongoDB executor")
	}

	wrapper := executor.wrapper
	if wrapper == nil {
		return nil, fmt.Errorf("invalid MongoDB connection")
	}

	// Get all collections in the database
	var filter bson.M
	if len(selectedCollections) > 0 && selectedCollections[0] != "ALL" {
		filter = bson.M{"name": bson.M{"$in": selectedCollections}}
	}

	collections, err := wrapper.Client.Database(wrapper.Database).ListCollections(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %v", err)
	}
	defer collections.Close(ctx)

	// Create a map to store all collections
	mongoSchema := MongoDBSchema{
		Collections: make(map[string]MongoDBCollection),
		Indexes:     make(map[string][]MongoDBIndex),
	}

	// Process each collection
	for collections.Next(ctx) {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			log.Printf("MongoDBDriver -> GetSchema -> Context cancelled: %v", err)
			return nil, err
		}

		var collInfo bson.M
		if err := collections.Decode(&collInfo); err != nil {
			log.Printf("MongoDBDriver -> GetSchema -> Error decoding collection info: %v", err)
			continue
		}

		collName, ok := collInfo["name"].(string)
		if !ok {
			log.Printf("MongoDBDriver -> GetSchema -> Invalid collection name")
			continue
		}

		log.Printf("MongoDBDriver -> GetSchema -> Processing collection: %s", collName)

		// Get collection details
		collection, err := d.getCollectionDetails(ctx, wrapper, collName)
		if err != nil {
			log.Printf("MongoDBDriver -> GetSchema -> Error getting collection details: %v", err)
			continue
		}

		// Get indexes for the collection
		indexes, err := d.getCollectionIndexes(ctx, wrapper, collName)
		if err != nil {
			log.Printf("MongoDBDriver -> GetSchema -> Error getting collection indexes: %v", err)
			continue
		}

		// Add to schema
		mongoSchema.Collections[collName] = collection
		mongoSchema.Indexes[collName] = indexes
	}

	if err := collections.Err(); err != nil {
		return nil, fmt.Errorf("error iterating collections: %v", err)
	}

	// Convert to generic SchemaInfo
	return d.convertToSchemaInfo(mongoSchema), nil
}

// getCollectionDetails retrieves details about a MongoDB collection
func (d *MongoDBDriver) getCollectionDetails(ctx context.Context, wrapper *MongoDBWrapper, collName string) (MongoDBCollection, error) {
	// Create a new collection
	collection := MongoDBCollection{
		Name:   collName,
		Fields: make(map[string]MongoDBField),
	}

	// Get document count
	count, err := wrapper.Client.Database(wrapper.Database).Collection(collName).CountDocuments(ctx, bson.M{})
	if err != nil {
		return collection, fmt.Errorf("failed to count documents: %v", err)
	}
	collection.DocumentCount = count

	// If collection is empty, return empty schema
	if count == 0 {
		return collection, nil
	}

	// Sample documents to infer schema
	sampleLimit := int64(50) // Sample up to 50 documents
	log.Printf("MongoDBDriver -> getCollectionDetails -> Will sample up to %d documents from collection %s for schema inference", sampleLimit, collName)

	opts := options.Find().SetLimit(sampleLimit)
	cursor, err := wrapper.Client.Database(wrapper.Database).Collection(collName).Find(ctx, bson.M{}, opts)
	if err != nil {
		return collection, fmt.Errorf("failed to sample documents: %v", err)
	}
	defer cursor.Close(ctx)

	// Process each document to infer schema
	var documents []bson.M
	if err := cursor.All(ctx, &documents); err != nil {
		return collection, fmt.Errorf("failed to decode documents: %v", err)
	}

	log.Printf("MongoDBDriver -> getCollectionDetails -> Retrieved exactly %d documents from collection %s for schema inference", len(documents), collName)

	// Store a sample document
	if len(documents) > 0 {
		collection.SampleDocument = documents[0]
	}

	// Infer schema from documents
	fields := make(map[string]MongoDBField)
	for _, doc := range documents {
		for key, value := range doc {
			field, exists := fields[key]
			if !exists {
				field = MongoDBField{
					Name:         key,
					IsRequired:   true,
					NestedFields: make(map[string]MongoDBField),
				}
			}

			// Determine field type
			fieldType := d.getMongoDBFieldType(value)
			if field.Type == "" {
				field.Type = fieldType
			} else if field.Type != fieldType && fieldType != "null" {
				// If types don't match, use a more generic type
				field.Type = "mixed"
			}

			// Check if it's an array
			if _, isArray := value.(primitive.A); isArray {
				field.IsArray = true
			}

			// Handle nested fields for objects
			if doc, isDoc := value.(bson.M); isDoc {
				for nestedKey, nestedValue := range doc {
					nestedField := MongoDBField{
						Name:       nestedKey,
						Type:       d.getMongoDBFieldType(nestedValue),
						IsRequired: true,
					}
					field.NestedFields[nestedKey] = nestedField
				}
			}

			fields[key] = field
		}
	}

	// Update required flag based on presence in all documents
	for _, doc := range documents {
		for key := range fields {
			if _, exists := doc[key]; !exists {
				field := fields[key]
				field.IsRequired = false
				fields[key] = field
			}
		}
	}

	collection.Fields = fields
	return collection, nil
}

// getCollectionIndexes retrieves indexes for a MongoDB collection
func (d *MongoDBDriver) getCollectionIndexes(ctx context.Context, wrapper *MongoDBWrapper, collName string) ([]MongoDBIndex, error) {
	// Get indexes
	cursor, err := wrapper.Client.Database(wrapper.Database).Collection(collName).Indexes().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %v", err)
	}
	defer cursor.Close(ctx)

	// Process each index
	var indexes []MongoDBIndex
	for cursor.Next(ctx) {
		var idx bson.M
		if err := cursor.Decode(&idx); err != nil {
			log.Printf("MongoDBDriver -> getCollectionIndexes -> Error decoding index: %v", err)
			continue
		}

		// Extract index information
		name, _ := idx["name"].(string)
		keys, _ := idx["key"].(bson.D)
		unique, _ := idx["unique"].(bool)
		sparse, _ := idx["sparse"].(bool)

		// Create index
		index := MongoDBIndex{
			Name:     name,
			Keys:     keys,
			IsUnique: unique,
			IsSparse: sparse,
		}

		indexes = append(indexes, index)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating indexes: %v", err)
	}

	return indexes, nil
}

// getMongoDBFieldType determines the type of a MongoDB field
func (d *MongoDBDriver) getMongoDBFieldType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case int32, int64, int:
		return "integer"
	case float64, float32:
		return "double"
	case bool:
		return "boolean"
	case primitive.DateTime:
		return "date"
	case primitive.ObjectID:
		return "objectId"
	case primitive.A:
		return "array"
	case bson.M, bson.D:
		return "object"
	case primitive.Binary:
		return "binary"
	default:
		return fmt.Sprintf("%T", value)
	}
}

// convertToSchemaInfo converts MongoDB schema to generic SchemaInfo
func (d *MongoDBDriver) convertToSchemaInfo(mongoSchema MongoDBSchema) *SchemaInfo {
	schema := &SchemaInfo{
		Tables:    make(map[string]TableSchema),
		Views:     make(map[string]ViewSchema),
		UpdatedAt: time.Now(),
	}

	// Convert collections to tables
	for collName, coll := range mongoSchema.Collections {
		tableSchema := TableSchema{
			Name:        collName,
			Columns:     make(map[string]ColumnInfo),
			Indexes:     make(map[string]IndexInfo),
			ForeignKeys: make(map[string]ForeignKey),
			Constraints: make(map[string]ConstraintInfo),
			RowCount:    coll.DocumentCount,
		}

		// Convert fields to columns
		for fieldName, field := range coll.Fields {
			columnType := field.Type
			if field.IsArray {
				columnType = "array<" + columnType + ">"
			}

			tableSchema.Columns[fieldName] = ColumnInfo{
				Name:         fieldName,
				Type:         columnType,
				IsNullable:   !field.IsRequired,
				DefaultValue: "",
				Comment:      "",
			}
		}

		// Convert indexes
		if indexes, ok := mongoSchema.Indexes[collName]; ok {
			for _, idx := range indexes {
				// Skip _id_ index as it's implicit
				if idx.Name == "_id_" {
					continue
				}

				// Extract column names from index keys
				columns := make([]string, 0, len(idx.Keys))
				for _, key := range idx.Keys {
					columns = append(columns, key.Key)
				}

				tableSchema.Indexes[idx.Name] = IndexInfo{
					Name:     idx.Name,
					Columns:  columns,
					IsUnique: idx.IsUnique,
				}
			}
		}

		schema.Tables[collName] = tableSchema
	}

	return schema
}

// GetTableChecksum calculates a checksum for a MongoDB collection
func (d *MongoDBDriver) GetTableChecksum(ctx context.Context, db DBExecutor, collection string) (string, error) {
	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		log.Printf("MongoDBDriver -> GetTableChecksum -> Context cancelled: %v", err)
		return "", err
	}

	// Get the MongoDB wrapper
	executor, ok := db.(*MongoDBExecutor)
	if !ok {
		return "", fmt.Errorf("invalid MongoDB executor")
	}

	wrapper := executor.wrapper
	if wrapper == nil {
		return "", fmt.Errorf("invalid MongoDB connection")
	}

	// Get collection schema
	coll, err := d.getCollectionDetails(ctx, wrapper, collection)
	if err != nil {
		return "", fmt.Errorf("failed to get collection details: %v", err)
	}

	// Get collection indexes
	indexes, err := d.getCollectionIndexes(ctx, wrapper, collection)
	if err != nil {
		return "", fmt.Errorf("failed to get collection indexes: %v", err)
	}

	// Create a checksum from collection fields
	fieldsChecksum := ""
	for fieldName, field := range coll.Fields {
		fieldType := field.Type
		if field.IsArray {
			fieldType = "array<" + fieldType + ">"
		}
		fieldsChecksum += fmt.Sprintf("%s:%s:%v,", fieldName, fieldType, field.IsRequired)
	}

	// Create a checksum from indexes
	indexesChecksum := ""
	for _, idx := range indexes {
		// Skip _id_ index as it's implicit
		if idx.Name == "_id_" {
			continue
		}

		// Extract key information
		keyInfo := ""
		for _, key := range idx.Keys {
			keyInfo += fmt.Sprintf("%s:%v,", key.Key, key.Value)
		}

		indexesChecksum += fmt.Sprintf("%s:%v:%v,", keyInfo, idx.IsUnique, idx.IsSparse)
	}

	// Combine checksums
	finalChecksum := fmt.Sprintf("%s:%s", fieldsChecksum, indexesChecksum)
	return utils.MD5Hash(finalChecksum), nil
}

// FetchExampleRecords fetches example records from a MongoDB collection
func (d *MongoDBDriver) FetchExampleRecords(ctx context.Context, db DBExecutor, collection string, limit int) ([]map[string]interface{}, error) {
	// Ensure limit is reasonable
	if limit <= 0 {
		limit = 3 // Default to 3 records
		log.Printf("MongoDBDriver -> FetchExampleRecords -> Using default limit of 3 records for collection %s", collection)
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
		log.Printf("MongoDBDriver -> FetchExampleRecords -> Capping limit to maximum of 10 records for collection %s", collection)
	} else {
		log.Printf("MongoDBDriver -> FetchExampleRecords -> Using requested limit of %d records for collection %s", limit, collection)
	}

	// Get the MongoDB wrapper
	executor, ok := db.(*MongoDBExecutor)
	if !ok {
		return nil, fmt.Errorf("invalid MongoDB executor")
	}

	wrapper := executor.wrapper
	if wrapper == nil {
		return nil, fmt.Errorf("invalid MongoDB connection")
	}

	// Fetch sample documents
	opts := options.Find().SetLimit(int64(limit))
	cursor, err := wrapper.Client.Database(wrapper.Database).Collection(collection).Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch example records: %v", err)
	}
	defer cursor.Close(ctx)

	// Process results
	var results []map[string]interface{}
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode document: %v", err)
		}

		// Convert BSON to map
		result := make(map[string]interface{})
		for k, v := range doc {
			// Convert MongoDB-specific types to JSON-friendly formats
			result[k] = d.convertMongoDBValue(v)
		}

		results = append(results, result)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating documents: %v", err)
	}

	// Log the exact number of records fetched
	log.Printf("MongoDBDriver -> FetchExampleRecords -> Retrieved exactly %d example records from collection %s", len(results), collection)

	// If no records found, return empty slice
	if len(results) == 0 {
		return []map[string]interface{}{}, nil
	}

	return results, nil
}

// convertMongoDBValue converts MongoDB-specific types to JSON-friendly formats
func (d *MongoDBDriver) convertMongoDBValue(value interface{}) interface{} {
	switch v := value.(type) {
	case primitive.ObjectID:
		return v.Hex()
	case primitive.DateTime:
		return time.Unix(0, int64(v)*int64(time.Millisecond)).Format(time.RFC3339)
	case primitive.A:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = d.convertMongoDBValue(item)
		}
		return result
	case bson.M:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = d.convertMongoDBValue(val)
		}
		return result
	case bson.D:
		result := make(map[string]interface{})
		for _, elem := range v {
			result[elem.Key] = d.convertMongoDBValue(elem.Value)
		}
		return result
	case primitive.Binary:
		return fmt.Sprintf("Binary(%d bytes)", len(v.Data))
	default:
		return v
	}
}

// MongoDBDriver implements the DatabaseDriver interface for MongoDB
type MongoDBDriver struct{}

// NewMongoDBDriver creates a new MongoDB driver
func NewMongoDBDriver() DatabaseDriver {
	return &MongoDBDriver{}
}

// Connect establishes a connection to a MongoDB database
func (d *MongoDBDriver) Connect(config ConnectionConfig) (*Connection, error) {
	var tempFiles []string
	log.Printf("MongoDBDriver -> Connect -> Connecting to MongoDB at %s:%v", config.Host, config.Port)

	var uri string
	port := "27017" // Default port for MongoDB

	// Check if we're using SRV records (mongodb+srv://)
	// Only check for .mongodb.net in non-encrypted hosts
	isSRV := false
	if !strings.Contains(config.Host, "+") && !strings.Contains(config.Host, "/") && !strings.Contains(config.Host, "=") {
		isSRV = strings.Contains(config.Host, ".mongodb.net")
	}

	protocol := "mongodb"
	if isSRV {
		protocol = "mongodb+srv"
	}

	// Validate port value if not using SRV
	if !isSRV && config.Port != nil {
		// Log the port value for debugging
		log.Printf("MongoDBDriver -> Connect -> Port value before validation: %v", *config.Port)

		// Check if port is empty
		if *config.Port == "" {
			log.Printf("MongoDBDriver -> Connect -> Port is empty, using default port 27017")
		} else {
			port = *config.Port

			// Skip port validation for encrypted ports (containing base64 characters)
			if strings.Contains(port, "+") || strings.Contains(port, "/") || strings.Contains(port, "=") {
				log.Printf("MongoDBDriver -> Connect -> Port appears to be encrypted, skipping validation")
			} else {
				// Verify port is numeric for non-encrypted ports
				if _, err := strconv.Atoi(port); err != nil {
					log.Printf("MongoDBDriver -> Connect -> Invalid port value: %v, error: %v", port, err)
					return nil, fmt.Errorf("invalid port value: %v, must be a number", port)
				}
			}
		}
	}

	// Base connection parameters with authentication
	if config.Username != nil && *config.Username != "" {
		// URL encode username and password to handle special characters
		encodedUsername := url.QueryEscape(*config.Username)
		var encodedPassword string
		if config.Password != nil {
			encodedPassword = url.QueryEscape(*config.Password)
		}

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
	log.Printf("MongoDBDriver -> Connect -> Connection URI: %s", maskedUri)

	// Add connection options
	if isSRV {
		uri += "?retryWrites=true&w=majority"
	} else {
		// For non-SRV connections, add a shorter server selection timeout
		uri += "?serverSelectionTimeoutMS=5000"
	}

	// Configure client options
	clientOptions := options.Client().ApplyURI(uri)

	// Set a shorter connection timeout for encrypted connections
	if strings.Contains(config.Host, "+") || strings.Contains(config.Host, "/") || strings.Contains(config.Host, "=") {
		clientOptions.SetConnectTimeout(5 * time.Second)
		clientOptions.SetServerSelectionTimeout(5 * time.Second)
		log.Printf("MongoDBDriver -> Connect -> Using shorter timeouts for encrypted connection")
	}

	// Configure SSL/TLS
	if config.UseSSL {
		// Fetch certificates from URLs
		certPath, keyPath, rootCertPath, certTempFiles, err := prepareCertificatesFromURLs(config)
		if err != nil {
			return nil, err
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
				return nil, fmt.Errorf("failed to load client certificates: %v", err)
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
				return nil, fmt.Errorf("failed to read root CA: %v", err)
			}

			rootCertPool := x509.NewCertPool()
			if ok := rootCertPool.AppendCertsFromPEM(rootCA); !ok {
				// Clean up temporary files
				for _, file := range tempFiles {
					os.Remove(file)
				}
				return nil, fmt.Errorf("failed to parse root CA certificate")
			}

			tlsConfig.RootCAs = rootCertPool
		}

		clientOptions.SetTLSConfig(tlsConfig)
	} else {
		// Disable SSL verification for encrypted connections
		clientOptions.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	}

	// Configure connection pool
	clientOptions.SetMaxPoolSize(25)
	clientOptions.SetMinPoolSize(5)
	clientOptions.SetMaxConnIdleTime(time.Hour)

	// Connect to MongoDB with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		log.Printf("MongoDBDriver -> Connect -> Error connecting to MongoDB: %v", err)
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Ping the database to verify connection
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		// Clean up temporary files
		for _, file := range tempFiles {
			os.Remove(file)
		}
		client.Disconnect(ctx)
		log.Printf("MongoDBDriver -> Connect -> Error pinging MongoDB: %v", err)
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	// Create a wrapper for the MongoDB client
	mongoWrapper := &MongoDBWrapper{
		Client:   client,
		Database: config.Database,
	}

	// Create a connection object
	conn := &Connection{
		DB:         nil, // MongoDB doesn't use GORM
		LastUsed:   time.Now(),
		Status:     StatusConnected,
		Config:     config,
		MongoDBObj: mongoWrapper, // Store MongoDB client in a custom field
		TempFiles:  tempFiles,    // Store temporary files for cleanup
		// Other fields will be set by the manager
	}

	log.Printf("MongoDBDriver -> Connect -> Successfully connected to MongoDB at %s:%v", config.Host, config.Port)
	return conn, nil
}

// Disconnect closes the MongoDB connection
func (d *MongoDBDriver) Disconnect(conn *Connection) error {
	log.Printf("MongoDBDriver -> Disconnect -> Disconnecting from MongoDB")

	// Get the MongoDB wrapper from the connection
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return fmt.Errorf("invalid MongoDB connection")
	}

	// Disconnect from MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := wrapper.Client.Disconnect(ctx)
	if err != nil {
		log.Printf("MongoDBDriver -> Disconnect -> Error disconnecting from MongoDB: %v", err)
		return fmt.Errorf("failed to disconnect from MongoDB: %v", err)
	}

	// Clean up temporary certificate files
	for _, file := range conn.TempFiles {
		os.Remove(file)
	}

	log.Printf("MongoDBDriver -> Disconnect -> Successfully disconnected from MongoDB")
	return nil
}

// Ping checks if the MongoDB connection is alive
func (d *MongoDBDriver) Ping(conn *Connection) error {
	log.Printf("MongoDBDriver -> Ping -> Pinging MongoDB")

	// Get the MongoDB wrapper from the connection
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return fmt.Errorf("invalid MongoDB connection")
	}

	// Ping MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := wrapper.Client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Printf("MongoDBDriver -> Ping -> Error pinging MongoDB: %v", err)
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	log.Printf("MongoDBDriver -> Ping -> Successfully pinged MongoDB")
	return nil
}

// IsAlive checks if the MongoDB connection is alive
func (d *MongoDBDriver) IsAlive(conn *Connection) bool {
	err := d.Ping(conn)
	return err == nil
}

// ExecuteQuery executes a MongoDB query
func (d *MongoDBDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult {
	startTime := time.Now()
	log.Printf("MongoDBDriver -> ExecuteQuery -> Executing MongoDB query: %s", query)

	// Get the MongoDB wrapper
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB connection",
				Code:    "CONNECTION_ERROR",
			},
		}
	}

	// Special case for createCollection which has a different format
	// Example: db.createCollection("collectionName", {...})
	if strings.Contains(query, "createCollection") {
		createCollectionRegex := regexp.MustCompile(`(?s)db\.createCollection\(["']([^"']+)["'](?:\s*,\s*)(.*)\)`)
		matches := createCollectionRegex.FindStringSubmatch(query)
		if len(matches) >= 3 {
			collectionName := matches[1]
			optionsStr := strings.TrimSpace(matches[2])

			log.Printf("MongoDBDriver -> ExecuteQuery -> Matched createCollection with collection: %s and options length: %d", collectionName, len(optionsStr))

			// Process the options
			var optionsMap bson.M
			if optionsStr != "" {
				// Process the options to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(optionsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process collection options: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				if err := json.Unmarshal([]byte(jsonStr), &optionsMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse collection options: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Check if collection already exists
			collections, err := wrapper.Client.Database(wrapper.Database).ListCollectionNames(ctx, bson.M{"name": collectionName})
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			// If collection already exists, return an error
			if len(collections) > 0 {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Collection '%s' already exists", collectionName),
						Code:    "COLLECTION_EXISTS",
					},
				}
			}

			// Create collection options
			var createOptions *options.CreateCollectionOptions
			if optionsMap != nil {
				// Convert validator to proper format if it exists
				if validator, ok := optionsMap["validator"]; ok {
					createOptions = &options.CreateCollectionOptions{
						Validator: validator,
					}
				}
			}

			// Execute the createCollection operation
			err = wrapper.Client.Database(wrapper.Database).CreateCollection(ctx, collectionName, createOptions)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to create collection: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			result := map[string]interface{}{
				"ok":      1,
				"message": fmt.Sprintf("Collection '%s' created successfully", collectionName),
			}

			// Convert the result to JSON
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
						Code:    "JSON_ERROR",
					},
				}
			}

			executionTime := int(time.Since(startTime).Milliseconds())
			log.Printf("MongoDBDriver -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

			return &QueryExecutionResult{
				Result:        result,
				ResultJSON:    string(resultJSON),
				ExecutionTime: executionTime,
			}
		}
	}

	// Handle database-level operations
	dbOperationRegex := regexp.MustCompile(`db\.(\w+)\(\s*(.*)\s*\)`)
	if dbOperationMatches := dbOperationRegex.FindStringSubmatch(query); len(dbOperationMatches) >= 2 {
		operation := dbOperationMatches[1]
		paramsStr := ""
		if len(dbOperationMatches) >= 3 {
			paramsStr = dbOperationMatches[2]
		}

		log.Printf("MongoDBDriver -> ExecuteQuery -> Matched database operation: %s with params: %s", operation, paramsStr)

		switch operation {
		case "getCollectionNames":
			// List all collections in the database
			collections, err := wrapper.Client.Database(wrapper.Database).ListCollectionNames(ctx, bson.M{})
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to list collections: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			// Convert the result to a map for consistent output
			result := map[string]interface{}{
				"collections": collections,
			}

			// Convert the result to JSON
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
						Code:    "JSON_ERROR",
					},
				}
			}

			executionTime := int(time.Since(startTime).Milliseconds())
			log.Printf("MongoDBDriver -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

			return &QueryExecutionResult{
				Result:        result,
				ResultJSON:    string(resultJSON),
				ExecutionTime: executionTime,
			}

		// Add more database-level operations here as needed
		default:
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Unsupported database operation: %s", operation),
					Code:    "UNSUPPORTED_OPERATION",
				},
			}
		}
	}

	// Parse the query
	// Example: db.collection.find({name: "John"})
	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "db") {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: db.collection.operation({...}) or db.operation(...)",
				Code:    "INVALID_QUERY",
			},
		}
	}

	collectionName := parts[1]
	operationWithParams := parts[2]

	// Split the operation and parameters
	// Example: find({name: "John"}) -> operation = find, params = {name: "John"}
	openParenIndex := strings.Index(operationWithParams, "(")
	closeParenIndex := strings.LastIndex(operationWithParams, ")")

	if openParenIndex == -1 || closeParenIndex == -1 || closeParenIndex <= openParenIndex {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: operation({...})",
				Code:    "INVALID_QUERY",
			},
		}
	}

	// Extract the operation and parameters
	operation := operationWithParams[:openParenIndex]
	paramsStr := operationWithParams[openParenIndex+1 : closeParenIndex]

	// Handle query modifiers like .limit(), .skip(), etc.
	modifiers := make(map[string]interface{})
	if closeParenIndex < len(operationWithParams)-1 {
		// There might be modifiers after the closing parenthesis
		modifiersStr := operationWithParams[closeParenIndex+1:]

		log.Printf("MongoDBDriver -> ExecuteQuery -> Modifiers string: %s", modifiersStr)

		// Extract limit modifier
		limitRegex := regexp.MustCompile(`\.limit\((\d+)\)`)
		if limitMatches := limitRegex.FindStringSubmatch(modifiersStr); len(limitMatches) > 1 {
			if limit, err := strconv.Atoi(limitMatches[1]); err == nil {
				modifiers["limit"] = limit
				log.Printf("MongoDBDriver -> ExecuteQuery -> Found limit modifier: %d", limit)
			}
		}

		// Extract skip modifier
		skipRegex := regexp.MustCompile(`\.skip\((\d+)\)`)
		if skipMatches := skipRegex.FindStringSubmatch(modifiersStr); len(skipMatches) > 1 {
			if skip, err := strconv.Atoi(skipMatches[1]); err == nil {
				modifiers["skip"] = skip
				log.Printf("MongoDBDriver -> ExecuteQuery -> Found skip modifier: %d", skip)
			}
		}

		// Extract sort modifier
		sortRegex := regexp.MustCompile(`\.sort\(([^)]+)\)`)
		if sortMatches := sortRegex.FindStringSubmatch(modifiersStr); len(sortMatches) > 1 {
			modifiers["sort"] = sortMatches[1]
			log.Printf("MongoDBDriver -> ExecuteQuery -> Found sort modifier: %s", sortMatches[1])
		}
	}

	// Get the MongoDB collection
	collection := wrapper.Client.Database(wrapper.Database).Collection(collectionName)

	// Check if the collection exists (except for dropCollection operation)
	if operation != "dropCollection" {
		// Check if collection exists by listing collections with a filter
		collections, err := collection.Database().ListCollectionNames(ctx, bson.M{"name": collectionName})
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		if len(collections) == 0 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Collection '%s' does not exist", collectionName),
					Code:    "COLLECTION_NOT_FOUND",
				},
			}
		}
	}

	var result interface{}
	var err error

	log.Printf("MongoDBDriver -> ExecuteQuery -> operation: %s", operation)
	// Execute the operation based on the type
	switch operation {
	case "find":
		// Parse the parameters as a BSON filter and projection
		// The parameters can be in two formats:
		// 1. find({filter}) - just a filter
		// 2. find({filter}, {projection}) - filter and projection

		var filter bson.M
		var projection bson.M

		// Check if we have both filter and projection
		if strings.Contains(paramsStr, "}, {") {
			// Split the parameters into filter and projection
			parts := strings.SplitN(paramsStr, "}, {", 2)
			if len(parts) == 2 {
				filterStr := parts[0] + "}"
				projectionStr := "{" + parts[1]

				log.Printf("MongoDBDriver -> ExecuteQuery -> Split parameters into filter: %s and projection: %s", filterStr, projectionStr)

				// Parse the filter
				if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
					// Try to handle MongoDB syntax with unquoted keys
					jsonFilterStr, err := processMongoDBQueryParams(filterStr)
					if err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}

					if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to parse filter: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}

					// Handle ObjectId in the filter
					if err := processObjectIds(filter); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process ObjectIds in filter: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}
				}

				// Parse the projection
				if err := json.Unmarshal([]byte(projectionStr), &projection); err != nil {
					// Try to handle MongoDB syntax with unquoted keys
					jsonProjectionStr, err := processMongoDBQueryParams(projectionStr)
					if err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process projection parameters: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}

					if err := json.Unmarshal([]byte(jsonProjectionStr), &projection); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to parse projection: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}
				}
			} else {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: "Invalid parameters format for find. Expected: find({filter}, {projection})",
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		} else {
			// Just a filter
			if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
				// Try to handle MongoDB syntax with unquoted keys and ObjectId
				log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

				// Process the query parameters to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(paramsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process query parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				log.Printf("MongoDBDriver -> ExecuteQuery -> Converted query: %s", jsonStr)

				if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Handle ObjectId in the filter
				if err := processObjectIds(filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Log the final filter for debugging
				filterJSON, _ := json.Marshal(filter)
				log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
			}
		}

		// Extract modifiers from the query string
		modifiers := extractModifiers(query)

		// If count() modifier is present, perform a count operation instead of find
		if modifiers.Count {
			// Execute the countDocuments operation
			count, err := collection.CountDocuments(ctx, filter)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to execute count operation: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			result = map[string]interface{}{
				"count": count,
			}
			break
		}

		// Create find options
		findOptions := options.Find()

		// Apply limit if specified
		if modifiers.Limit > 0 {
			findOptions.SetLimit(modifiers.Limit)
		}

		// Apply skip if specified
		if modifiers.Skip > 0 {
			findOptions.SetSkip(modifiers.Skip)
		}

		// Apply sort if specified
		if modifiers.Sort != "" {
			var sortDoc bson.D
			sortJSON := modifiers.Sort

			// Process the sort expression to handle MongoDB syntax
			if !strings.HasPrefix(sortJSON, "{") {
				sortJSON = fmt.Sprintf(`{"%s": 1}`, sortJSON)
			}

			// Parse the sort document
			var sortMap bson.M
			if err := json.Unmarshal([]byte(sortJSON), &sortMap); err != nil {
				jsonStr, err := processMongoDBQueryParams(sortJSON)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process sort parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				if err := json.Unmarshal([]byte(jsonStr), &sortMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse sort parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Convert the sort map to a bson.D
			for k, v := range sortMap {
				sortDoc = append(sortDoc, bson.E{Key: k, Value: v})
			}

			findOptions.SetSort(sortDoc)
		}

		// Apply projection if specified from the parameters or modifiers
		if projection != nil {
			// Convert the projection map to a bson.D
			var projectionDoc bson.D
			for k, v := range projection {
				projectionDoc = append(projectionDoc, bson.E{Key: k, Value: v})
			}
			findOptions.SetProjection(projectionDoc)
		} else if modifiers.Projection != "" {
			var projectionDoc bson.D
			projectionJSON := modifiers.Projection

			// Process the projection expression to handle MongoDB syntax
			if !strings.HasPrefix(projectionJSON, "{") {
				projectionJSON = fmt.Sprintf(`{"%s": 1}`, projectionJSON)
			}

			// Parse the projection document
			var projectionMap bson.M
			if err := json.Unmarshal([]byte(projectionJSON), &projectionMap); err != nil {
				jsonStr, err := processMongoDBQueryParams(projectionJSON)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process projection parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				if err := json.Unmarshal([]byte(jsonStr), &projectionMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse projection parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Convert the projection map to a bson.D
			for k, v := range projectionMap {
				projectionDoc = append(projectionDoc, bson.E{Key: k, Value: v})
			}

			findOptions.SetProjection(projectionDoc)
		}

		// Execute the find operation
		cursor, err := collection.Find(ctx, filter, findOptions)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute find operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}
		defer cursor.Close(ctx)

		// Decode the results
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to decode find results: %v", err),
					Code:    "DECODE_ERROR",
				},
			}
		}

		result = results

	case "findOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process query parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted query: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Execute the findOne operation
		var doc bson.M
		err = collection.FindOne(ctx, filter).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// No documents found, return empty result
				result = nil
			} else {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to execute findOne operation: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}
		} else {
			result = doc
		}

	case "insertOne":
		// Parse the parameters as a BSON document
		var document bson.M
		if err := json.Unmarshal([]byte(paramsStr), &document); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and special types like Date
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB document: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process document: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted document: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &document); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse document: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId and other special types in the document
			if err := processObjectIds(document); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the insertOne operation
		insertResult, err := collection.InsertOne(ctx, document)
		if err != nil {
			// Check for duplicate key error
			if mongo.IsDuplicateKeyError(err) {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: "Document with the same unique key already exists",
						Code:    "DUPLICATE_KEY",
					},
				}
			}

			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"insertedId": insertResult.InsertedID,
		}

	case "insertMany":
		// Parse the parameters as an array of BSON documents
		var documents []interface{}
		if err := json.Unmarshal([]byte(paramsStr), &documents); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB documents: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process documents: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted documents: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &documents); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse documents after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the insertMany operation
		insertResult, err := collection.InsertMany(ctx, documents)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"insertedIds":   insertResult.InsertedIDs,
			"insertedCount": len(insertResult.InsertedIDs),
		}

	case "updateOne":
		// Parse the parameters as filter and update
		// Expected format: ({filter}, {update})
		params := strings.SplitN(paramsStr, ",", 2)
		if len(params) != 2 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: "Invalid parameters for updateOne. Expected: ({filter}, {update})",
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Process filter with MongoDB syntax
		filterStr := params[0]
		updateStr := params[1]

		// Process the filter to handle MongoDB syntax
		var filter bson.M
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", filterStr)

			// Process the query parameters to handle MongoDB syntax
			jsonFilterStr, err := processMongoDBQueryParams(filterStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted filter: %s", jsonFilterStr)

			if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Process update with MongoDB syntax
		var update bson.M
		if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB update: %s", updateStr)

			// Process the query parameters to handle MongoDB syntax
			jsonUpdateStr, err := processMongoDBQueryParams(updateStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process update parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted update: %s", jsonUpdateStr)

			if err := json.Unmarshal([]byte(jsonUpdateStr), &update); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse update after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the updateOne operation
		updateResult, err := collection.UpdateOne(ctx, filter, update)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute updateOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// Check if any document was matched
		if updateResult.MatchedCount == 0 {
			log.Printf("MongoDBDriver -> ExecuteQuery -> No document matched the filter criteria for updateOne")
		}

		result = map[string]interface{}{
			"matchedCount":  updateResult.MatchedCount,
			"modifiedCount": updateResult.ModifiedCount,
			"upsertedId":    updateResult.UpsertedID,
		}

	case "updateMany":
		// Parse the parameters as filter and update
		// Expected format: ({filter}, {update})
		params := strings.SplitN(paramsStr, ",", 2)
		if len(params) != 2 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: "Invalid parameters for updateMany. Expected: ({filter}, {update})",
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Process filter with MongoDB syntax
		filterStr := params[0]
		updateStr := params[1]

		// Process the filter to handle MongoDB syntax
		var filter bson.M
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", filterStr)

			// Process the query parameters to handle MongoDB syntax
			jsonFilterStr, err := processMongoDBQueryParams(filterStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted filter: %s", jsonFilterStr)

			if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Process update with MongoDB syntax
		var update bson.M
		if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB update: %s", updateStr)

			// Process the query parameters to handle MongoDB syntax
			jsonUpdateStr, err := processMongoDBQueryParams(updateStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process update parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted update: %s", jsonUpdateStr)

			if err := json.Unmarshal([]byte(jsonUpdateStr), &update); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse update after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the updateMany operation
		updateResult, err := collection.UpdateMany(ctx, filter, update)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute updateMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"matchedCount":  updateResult.MatchedCount,
			"modifiedCount": updateResult.ModifiedCount,
			"upsertedId":    updateResult.UpsertedID,
		}

	case "deleteOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process query parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted query: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Execute the deleteOne operation
		deleteResult, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// Check if any document was deleted
		if deleteResult.DeletedCount == 0 {
			log.Printf("MongoDBDriver -> ExecuteQuery -> No document matched the filter criteria for deleteOne")
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "deleteMany":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and operators like $or
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax

			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted filter: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final filter after conversion: %s", string(filterJSON))
		}

		// Execute the deleteMany operation
		deleteResult, err := collection.DeleteMany(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "aggregate":
		// Parse the parameters as a pipeline
		var pipeline []bson.M
		if err := json.Unmarshal([]byte(paramsStr), &pipeline); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB aggregation pipeline: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process aggregation pipeline: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBDriver -> ExecuteQuery -> Converted aggregation pipeline: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &pipeline); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse aggregation pipeline after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Process ObjectIds in each stage of the pipeline
			for _, stage := range pipeline {
				if err := processObjectIds(stage); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds in aggregation pipeline: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Log the final pipeline for debugging
			pipelineJSON, _ := json.Marshal(pipeline)
			log.Printf("MongoDBDriver -> ExecuteQuery -> Final aggregation pipeline after ObjectId conversion: %s", string(pipelineJSON))
		}

		// Convert []bson.M to mongo.Pipeline
		mongoPipeline := make(mongo.Pipeline, len(pipeline))
		for i, stage := range pipeline {
			// Convert each stage to bson.D
			stageD := bson.D{}
			for k, v := range stage {
				stageD = append(stageD, bson.E{Key: k, Value: v})
			}
			mongoPipeline[i] = stageD
		}

		// Execute the aggregate operation
		cursor, err := collection.Aggregate(ctx, mongoPipeline)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute aggregate operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}
		defer cursor.Close(ctx)

		// Decode the results
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to decode aggregation results: %v", err),
					Code:    "DECODE_ERROR",
				},
			}
		}

		result = results

	case "countDocuments":
		// Parse the parameters as a BSON filter
		var filter bson.M

		// Handle empty parameters for countDocuments()
		if strings.TrimSpace(paramsStr) == "" {
			// Use an empty filter to count all documents
			filter = bson.M{}
		} else {
			// Parse the provided filter
			if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
				// Try to handle MongoDB syntax with unquoted keys
				log.Printf("MongoDBDriver -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", paramsStr)

				// Process the query parameters to handle MongoDB syntax

				jsonStr, err := processMongoDBQueryParams(paramsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				log.Printf("MongoDBDriver -> ExecuteQuery -> Converted filter: %s", jsonStr)

				if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse filter: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Handle ObjectId in the filter
				if err := processObjectIds(filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}
		}

		// Execute the countDocuments operation
		count, err := collection.CountDocuments(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute countDocuments operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"count": count,
		}

	case "createCollection":
		// Execute the createCollection operation with default options
		// We're simplifying this implementation to avoid complex option handling
		err := collection.Database().CreateCollection(ctx, collectionName)
		if err != nil {
			// Check if collection already exists
			if strings.Contains(err.Error(), "already exists") {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Collection '%s' already exists", collectionName),
						Code:    "COLLECTION_EXISTS",
					},
				}
			}

			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to create collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' created successfully", collectionName),
		}

	case "dropCollection":
		// Execute the dropCollection operation
		err := collection.Drop(ctx)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to drop collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' dropped successfully", collectionName),
		}

	case "drop":
		// Check if collection exists before dropping
		collections, err := wrapper.Client.Database(wrapper.Database).ListCollectionNames(ctx, bson.M{"name": collectionName})
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// If collection doesn't exist, return an error
		if len(collections) == 0 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Collection '%s' does not exist", collectionName),
					Code:    "COLLECTION_NOT_FOUND",
				},
			}
		}

		// Execute the drop operation
		err = collection.Drop(ctx)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to drop collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' dropped successfully", collectionName),
		}

	default:
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Unsupported MongoDB operation: %s", operation),
				Code:    "UNSUPPORTED_OPERATION",
			},
		}
	}

	// Convert the result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
				Code:    "JSON_ERROR",
			},
		}
	}

	var resultMap map[string]interface{}
	if tempResultMap, ok := result.(map[string]interface{}); ok {
		// Create a result map
		resultMap = tempResultMap
	} else {
		resultMap = map[string]interface{}{
			"results": result,
		}
	}

	executionTime := int(time.Since(startTime).Milliseconds())
	log.Printf("MongoDBDriver -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

	return &QueryExecutionResult{
		Result:        resultMap,
		ResultJSON:    string(resultJSON),
		ExecutionTime: executionTime,
	}
}

// BeginTx begins a MongoDB transaction
func (d *MongoDBDriver) BeginTx(ctx context.Context, conn *Connection) Transaction {
	log.Printf("MongoDBDriver -> BeginTx -> Beginning MongoDB transaction")

	// Get the MongoDB wrapper
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		log.Printf("MongoDBDriver -> BeginTx -> Invalid MongoDB connection")
		return &MongoDBTransaction{
			Error: fmt.Errorf("invalid MongoDB connection"),
			// Session is nil here, but that's expected since we have an error
		}
	}

	// Ensure the client is not nil
	if wrapper.Client == nil {
		log.Printf("MongoDBDriver -> BeginTx -> MongoDB client is nil")
		return &MongoDBTransaction{
			Error:   fmt.Errorf("MongoDB client is nil"),
			Wrapper: wrapper,
			// Session is nil here, but that's expected since we have an error
		}
	}

	// Verify the connection is alive before starting a transaction
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := wrapper.Client.Ping(pingCtx, readpref.Primary()); err != nil {
		log.Printf("MongoDBDriver -> BeginTx -> MongoDB connection is not alive: %v", err)
		return &MongoDBTransaction{
			Error:   fmt.Errorf("MongoDB connection is not alive: %v", err),
			Wrapper: wrapper,
		}
	}

	// Start a new session with retry logic
	var session mongo.Session
	var err error

	// Try up to 3 times to start a session
	for attempts := 0; attempts < 3; attempts++ {
		session, err = wrapper.Client.StartSession()
		if err == nil {
			break
		}
		log.Printf("MongoDBDriver -> BeginTx -> Error starting MongoDB session (attempt %d/3): %v", attempts+1, err)
		time.Sleep(500 * time.Millisecond) // Wait before retrying
	}

	if err != nil {
		log.Printf("MongoDBDriver -> BeginTx -> Failed to start MongoDB session after retries: %v", err)
		return &MongoDBTransaction{
			Error:   fmt.Errorf("failed to start MongoDB session after retries: %v", err),
			Wrapper: wrapper,
		}
	}

	// Start a transaction with retry logic
	for attempts := 0; attempts < 3; attempts++ {
		err = session.StartTransaction()
		if err == nil {
			break
		}
		log.Printf("MongoDBDriver -> BeginTx -> Error starting MongoDB transaction (attempt %d/3): %v", attempts+1, err)
		time.Sleep(500 * time.Millisecond) // Wait before retrying
	}

	if err != nil {
		log.Printf("MongoDBDriver -> BeginTx -> Failed to start MongoDB transaction after retries: %v", err)
		session.EndSession(ctx)
		return &MongoDBTransaction{
			Error:   fmt.Errorf("failed to start MongoDB transaction after retries: %v", err),
			Wrapper: wrapper,
		}
	}

	// Create a new transaction object
	tx := &MongoDBTransaction{
		Session: session,
		Wrapper: wrapper,
		Error:   nil,
	}

	log.Printf("MongoDBDriver -> BeginTx -> MongoDB transaction started successfully")
	return tx
}

// Commit commits a MongoDB transaction
func (tx *MongoDBTransaction) Commit() error {
	log.Printf("MongoDBTransaction -> Commit -> Committing MongoDB transaction")

	// Check if the session is nil (which can happen if there was an error creating the transaction)
	if tx.Session == nil {
		log.Printf("MongoDBTransaction -> Commit -> No session to commit (session is nil)")
		if tx.Error != nil {
			log.Printf("MongoDBTransaction -> Commit -> Original error: %v", tx.Error)
			return fmt.Errorf("cannot commit transaction: %v", tx.Error)
		}
		return fmt.Errorf("cannot commit transaction: session is nil")
	}

	// Check if the wrapper or client is nil
	if tx.Wrapper == nil || tx.Wrapper.Client == nil {
		log.Printf("MongoDBTransaction -> Commit -> Wrapper or client is nil")
		return fmt.Errorf("cannot commit: wrapper or client is nil")
	}

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		log.Printf("MongoDBTransaction -> Commit -> Cannot commit with error: %v", tx.Error)
		// End the session if it exists
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tx.Session.EndSession(ctx)
		return fmt.Errorf("cannot commit transaction with error: %v", tx.Error)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Commit the transaction with retry logic
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = tx.Session.CommitTransaction(ctx)
		if err == nil {
			break
		}
		log.Printf("MongoDBTransaction -> Commit -> Error committing transaction (attempt %d/3): %v", attempts+1, err)
		time.Sleep(500 * time.Millisecond) // Wait before retrying
	}

	if err != nil {
		log.Printf("MongoDBTransaction -> Commit -> Failed to commit transaction after retries: %v", err)
		// Still try to end the session even if commit fails
		tx.Session.EndSession(ctx)
		return fmt.Errorf("failed to commit MongoDB transaction: %v", err)
	}

	// End the session
	tx.Session.EndSession(ctx)

	log.Printf("MongoDBTransaction -> Commit -> MongoDB transaction committed successfully")
	return nil
}

// Rollback rolls back a MongoDB transaction
func (tx *MongoDBTransaction) Rollback() error {
	log.Printf("MongoDBTransaction -> Rollback -> Rolling back MongoDB transaction")

	// Check if the session is nil (which can happen if there was an error creating the transaction)
	if tx.Session == nil {
		log.Printf("MongoDBTransaction -> Rollback -> No session to roll back (session is nil)")
		if tx.Error != nil {
			log.Printf("MongoDBTransaction -> Rollback -> Original error: %v", tx.Error)
			return tx.Error
		}
		return nil
	}

	// Check if the wrapper or client is nil
	if tx.Wrapper == nil || tx.Wrapper.Client == nil {
		log.Printf("MongoDBTransaction -> Rollback -> Wrapper or client is nil")
		return fmt.Errorf("cannot rollback: wrapper or client is nil")
	}

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		// If there was an error starting the transaction, just end the session
		log.Printf("MongoDBTransaction -> Rollback -> Rolling back with error: %v", tx.Error)

		// Use a timeout context for ending the session
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tx.Session.EndSession(ctx)
		return tx.Error
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Abort the transaction with retry logic
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = tx.Session.AbortTransaction(ctx)
		if err == nil {
			break
		}
		log.Printf("MongoDBTransaction -> Rollback -> Error aborting transaction (attempt %d/3): %v", attempts+1, err)
		time.Sleep(500 * time.Millisecond) // Wait before retrying
	}

	if err != nil {
		log.Printf("MongoDBTransaction -> Rollback -> Failed to abort transaction after retries: %v", err)
		// Still try to end the session even if abort fails
		tx.Session.EndSession(ctx)
		return fmt.Errorf("failed to abort MongoDB transaction: %v", err)
	}

	// End the session
	tx.Session.EndSession(ctx)

	log.Printf("MongoDBTransaction -> Rollback -> MongoDB transaction rolled back successfully")
	return nil
}

// MongoDBWrapper wraps a MongoDB client
type MongoDBWrapper struct {
	Client   *mongo.Client
	Database string
}

// MongoDBTransaction implements the Transaction interface for MongoDB
type MongoDBTransaction struct {
	Session mongo.Session
	Wrapper *MongoDBWrapper
	Error   error
}

// ExecuteQuery executes a MongoDB query within a transaction
func (tx *MongoDBTransaction) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string, findCount bool) *QueryExecutionResult {
	log.Printf("MongoDBTransaction -> ExecuteQuery -> Executing MongoDB query in transaction: %s", query)
	startTime := time.Now()

	// Check if the session is nil (which can happen if there was an error creating the transaction)
	if tx.Session == nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Cannot execute query: session is nil")
		errorMsg := "Cannot execute query: transaction session is nil"
		if tx.Error != nil {
			errorMsg = fmt.Sprintf("Cannot execute query: %v", tx.Error)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Original error: %v", tx.Error)
		}
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: errorMsg,
				Code:    "TRANSACTION_ERROR",
			},
		}
	}

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Transaction error: %v", tx.Error)
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Transaction error: %v", tx.Error),
				Code:    "TRANSACTION_ERROR",
			},
		}
	}

	// Check if the wrapper is nil
	if tx.Wrapper == nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> Wrapper is nil")
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Transaction wrapper is nil",
				Code:    "TRANSACTION_ERROR",
			},
		}
	}

	// Verify the client is not nil
	if tx.Wrapper.Client == nil {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB client is nil")
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "MongoDB client is nil",
				Code:    "TRANSACTION_ERROR",
			},
		}
	}

	// Verify the session is still valid by checking if the client is still connected
	// This is a lightweight check that doesn't require a full ping
	if tx.Wrapper.Client.NumberSessionsInProgress() == 0 {
		log.Printf("MongoDBTransaction -> ExecuteQuery -> No active sessions, session may have expired")
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Transaction session may have expired",
				Code:    "TRANSACTION_ERROR",
			},
		}
	}

	// Special case for createCollection which has a different format
	// Example: db.createCollection("collectionName", {...})
	if strings.Contains(query, "createCollection") {
		createCollectionRegex := regexp.MustCompile(`(?s)db\.createCollection\(["']([^"']+)["'](?:\s*,\s*)(.*)\)`)
		matches := createCollectionRegex.FindStringSubmatch(query)
		if len(matches) >= 3 {
			collectionName := matches[1]
			optionsStr := strings.TrimSpace(matches[2])

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Matched createCollection with collection: %s and options length: %d", collectionName, len(optionsStr))

			// Process the options
			var optionsMap bson.M
			if optionsStr != "" {
				// Process the options to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(optionsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process collection options: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				if err := json.Unmarshal([]byte(jsonStr), &optionsMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse collection options: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Check if collection already exists
			collections, err := tx.Wrapper.Client.Database(tx.Wrapper.Database).ListCollectionNames(ctx, bson.M{"name": collectionName})
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			// If collection already exists, return an error
			if len(collections) > 0 {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Collection '%s' already exists", collectionName),
						Code:    "COLLECTION_EXISTS",
					},
				}
			}

			// Create collection options
			var createOptions *options.CreateCollectionOptions
			if optionsMap != nil {
				// Convert validator to proper format if it exists
				if validator, ok := optionsMap["validator"]; ok {
					createOptions = &options.CreateCollectionOptions{
						Validator: validator,
					}
				}
			}

			// Execute the createCollection operation
			err = tx.Wrapper.Client.Database(tx.Wrapper.Database).CreateCollection(ctx, collectionName, createOptions)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to create collection: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			result := map[string]interface{}{
				"ok":      1,
				"message": fmt.Sprintf("Collection '%s' created successfully", collectionName),
			}

			// Convert the result to JSON
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
						Code:    "JSON_ERROR",
					},
				}
			}

			executionTime := int(time.Since(startTime).Milliseconds())
			log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

			return &QueryExecutionResult{
				Result:        result,
				ResultJSON:    string(resultJSON),
				ExecutionTime: executionTime,
			}
		}
	}

	// Handle database-level operations
	dbOperationRegex := regexp.MustCompile(`db\.(\w+)\(\s*(.*)\s*\)`)
	if dbOperationMatches := dbOperationRegex.FindStringSubmatch(query); len(dbOperationMatches) >= 2 {
		operation := dbOperationMatches[1]
		paramsStr := ""
		if len(dbOperationMatches) >= 3 {
			paramsStr = dbOperationMatches[2]
		}

		log.Printf("MongoDBTransaction -> ExecuteQuery -> Matched database operation: %s with params: %s", operation, paramsStr)

		switch operation {
		case "getCollectionNames":
			// List all collections in the database
			collections, err := tx.Wrapper.Client.Database(tx.Wrapper.Database).ListCollectionNames(ctx, bson.M{})
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to list collections: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			// Convert the result to a map for consistent output
			result := map[string]interface{}{
				"collections": collections,
			}

			// Convert the result to JSON
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
						Code:    "JSON_ERROR",
					},
				}
			}

			executionTime := int(time.Since(startTime).Milliseconds())
			log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

			return &QueryExecutionResult{
				Result:        result,
				ResultJSON:    string(resultJSON),
				ExecutionTime: executionTime,
			}

		// Add more database-level operations here as needed
		default:
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Unsupported database operation: %s", operation),
					Code:    "UNSUPPORTED_OPERATION",
				},
			}
		}
	}

	// Parse the MongoDB query
	// MongoDB queries are expected in the format: db.collection.operation({...})
	// For example: db.users.find({name: "John"})
	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "db") {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: db.collection.operation({...}) or db.operation(...)",
				Code:    "INVALID_QUERY",
			},
		}
	}

	collectionName := parts[1]
	operationWithParams := parts[2]

	// Split the operation and parameters
	// Example: find({name: "John"}) -> operation = find, params = {name: "John"}
	openParenIndex := strings.Index(operationWithParams, "(")
	closeParenIndex := strings.LastIndex(operationWithParams, ")")

	if openParenIndex == -1 || closeParenIndex == -1 || closeParenIndex <= openParenIndex {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: operation({...})",
				Code:    "INVALID_QUERY",
			},
		}
	}

	// Extract the operation and parameters
	operation := operationWithParams[:openParenIndex]
	paramsStr := operationWithParams[openParenIndex+1 : closeParenIndex]

	// Handle query modifiers like .limit(), .skip(), etc.
	modifiers := make(map[string]interface{})
	if closeParenIndex < len(operationWithParams)-1 {
		// There might be modifiers after the closing parenthesis
		modifiersStr := operationWithParams[closeParenIndex+1:]

		log.Printf("MongoDBTransaction -> ExecuteQuery -> Modifiers string: %s", modifiersStr)

		// Extract limit modifier
		limitRegex := regexp.MustCompile(`\.limit\((\d+)\)`)
		if limitMatches := limitRegex.FindStringSubmatch(modifiersStr); len(limitMatches) > 1 {
			if limit, err := strconv.Atoi(limitMatches[1]); err == nil {
				modifiers["limit"] = limit
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Found limit modifier: %d", limit)
			}
		}

		// Extract skip modifier
		skipRegex := regexp.MustCompile(`\.skip\((\d+)\)`)
		if skipMatches := skipRegex.FindStringSubmatch(modifiersStr); len(skipMatches) > 1 {
			if skip, err := strconv.Atoi(skipMatches[1]); err == nil {
				modifiers["skip"] = skip
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Found skip modifier: %d", skip)
			}
		}

		// Extract sort modifier
		sortRegex := regexp.MustCompile(`\.sort\(([^)]+)\)`)
		if sortMatches := sortRegex.FindStringSubmatch(modifiersStr); len(sortMatches) > 1 {
			modifiers["sort"] = sortMatches[1]
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Found sort modifier: %s", sortMatches[1])
		}
	}

	// Get the MongoDB collection
	collection := tx.Wrapper.Client.Database(tx.Wrapper.Database).Collection(collectionName)

	// Check if the collection exists (except for dropCollection operation)
	if operation != "dropCollection" {
		// Check if collection exists by listing collections with a filter
		collections, err := collection.Database().ListCollectionNames(ctx, bson.M{"name": collectionName})
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		if len(collections) == 0 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Collection '%s' does not exist", collectionName),
					Code:    "COLLECTION_NOT_FOUND",
				},
			}
		}
	}

	var result interface{}
	var err error

	log.Printf("MongoDBTransaction -> ExecuteQuery -> operation: %s", operation)
	// Execute the operation based on the type
	switch operation {
	case "find":
		// Parse the parameters as a BSON filter and projection
		// The parameters can be in two formats:
		// 1. find({filter}) - just a filter
		// 2. find({filter}, {projection}) - filter and projection

		var filter bson.M
		var projection bson.M

		// Check if we have both filter and projection
		if strings.Contains(paramsStr, "}, {") {
			// Split the parameters into filter and projection
			parts := strings.SplitN(paramsStr, "}, {", 2)
			if len(parts) == 2 {
				filterStr := parts[0] + "}"
				projectionStr := "{" + parts[1]

				log.Printf("MongoDBTransaction -> ExecuteQuery -> Split parameters into filter: %s and projection: %s", filterStr, projectionStr)

				// Parse the filter
				if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
					// Try to handle MongoDB syntax with unquoted keys
					jsonFilterStr, err := processMongoDBQueryParams(filterStr)
					if err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}

					if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to parse filter: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}

					// Handle ObjectId in the filter
					if err := processObjectIds(filter); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process ObjectIds in filter: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}
				}

				// Parse the projection
				if err := json.Unmarshal([]byte(projectionStr), &projection); err != nil {
					// Try to handle MongoDB syntax with unquoted keys
					jsonProjectionStr, err := processMongoDBQueryParams(projectionStr)
					if err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to process projection parameters: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}

					if err := json.Unmarshal([]byte(jsonProjectionStr), &projection); err != nil {
						return &QueryExecutionResult{
							Error: &dtos.QueryError{
								Message: fmt.Sprintf("Failed to parse projection: %v", err),
								Code:    "INVALID_PARAMETERS",
							},
						}
					}
				}
			} else {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: "Invalid parameters format for find. Expected: find({filter}, {projection})",
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		} else {
			// Just a filter
			if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
				// Try to handle MongoDB syntax with unquoted keys and ObjectId
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

				// Process the query parameters to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(paramsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process query parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted query: %s", jsonStr)

				if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Handle ObjectId in the filter
				if err := processObjectIds(filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Log the final filter for debugging
				filterJSON, _ := json.Marshal(filter)
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
			}
		}

		// Extract modifiers from the query string
		modifiers := extractModifiers(query)

		// If count() modifier is present, perform a count operation instead of find
		if modifiers.Count {
			// Execute the countDocuments operation
			count, err := collection.CountDocuments(ctx, filter)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to execute count operation: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}

			result = map[string]interface{}{
				"count": count,
			}
			break
		}

		// Create find options
		findOptions := options.Find()

		// Apply limit if specified
		if modifiers.Limit > 0 {
			findOptions.SetLimit(modifiers.Limit)
		}

		// Apply skip if specified
		if modifiers.Skip > 0 {
			findOptions.SetSkip(modifiers.Skip)
		}

		// Apply sort if specified
		if modifiers.Sort != "" {
			var sortDoc bson.D
			sortJSON := modifiers.Sort

			// Process the sort expression to handle MongoDB syntax
			if !strings.HasPrefix(sortJSON, "{") {
				sortJSON = fmt.Sprintf(`{"%s": 1}`, sortJSON)
			}

			// Parse the sort document
			var sortMap bson.M
			if err := json.Unmarshal([]byte(sortJSON), &sortMap); err != nil {
				jsonStr, err := processMongoDBQueryParams(sortJSON)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process sort parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				if err := json.Unmarshal([]byte(jsonStr), &sortMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse sort parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Convert the sort map to a bson.D
			for k, v := range sortMap {
				sortDoc = append(sortDoc, bson.E{Key: k, Value: v})
			}

			findOptions.SetSort(sortDoc)
		}

		// Apply projection if specified from the parameters or modifiers
		if projection != nil {
			// Convert the projection map to a bson.D
			var projectionDoc bson.D
			for k, v := range projection {
				projectionDoc = append(projectionDoc, bson.E{Key: k, Value: v})
			}
			findOptions.SetProjection(projectionDoc)
		} else if modifiers.Projection != "" {
			var projectionDoc bson.D
			projectionJSON := modifiers.Projection

			// Process the projection expression to handle MongoDB syntax
			if !strings.HasPrefix(projectionJSON, "{") {
				projectionJSON = fmt.Sprintf(`{"%s": 1}`, projectionJSON)
			}

			// Parse the projection document
			var projectionMap bson.M
			if err := json.Unmarshal([]byte(projectionJSON), &projectionMap); err != nil {
				jsonStr, err := processMongoDBQueryParams(projectionJSON)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process projection parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				if err := json.Unmarshal([]byte(jsonStr), &projectionMap); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse projection parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Convert the projection map to a bson.D
			for k, v := range projectionMap {
				projectionDoc = append(projectionDoc, bson.E{Key: k, Value: v})
			}

			findOptions.SetProjection(projectionDoc)
		}

		// Execute the find operation
		cursor, err := collection.Find(ctx, filter, findOptions)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute find operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}
		defer cursor.Close(ctx)

		// Decode the results
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to decode find results: %v", err),
					Code:    "DECODE_ERROR",
				},
			}
		}

		result = results

	case "findOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process query parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted query: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Execute the findOne operation
		var doc bson.M
		err = collection.FindOne(ctx, filter).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// No documents found, return empty result
				result = nil
			} else {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to execute findOne operation: %v", err),
						Code:    "EXECUTION_ERROR",
					},
				}
			}
		} else {
			result = doc
		}

	case "insertOne":
		// Parse the parameters as a BSON document
		var document bson.M
		if err := json.Unmarshal([]byte(paramsStr), &document); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and special types like Date
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB document: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process document: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted document: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &document); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse document: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId and other special types in the document
			if err := processObjectIds(document); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the insertOne operation
		insertResult, err := collection.InsertOne(ctx, document)
		if err != nil {
			// Check for duplicate key error
			if mongo.IsDuplicateKeyError(err) {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: "Document with the same unique key already exists",
						Code:    "DUPLICATE_KEY",
					},
				}
			}

			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"insertedId": insertResult.InsertedID,
		}

	case "insertMany":
		// Parse the parameters as an array of BSON documents
		var documents []interface{}
		if err := json.Unmarshal([]byte(paramsStr), &documents); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB documents: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process documents: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted documents: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &documents); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse documents after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the insertMany operation
		insertResult, err := collection.InsertMany(ctx, documents)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"insertedIds":   insertResult.InsertedIDs,
			"insertedCount": len(insertResult.InsertedIDs),
		}

	case "updateOne":
		// Parse the parameters as filter and update
		// Expected format: ({filter}, {update})
		params := strings.SplitN(paramsStr, ",", 2)
		if len(params) != 2 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: "Invalid parameters for updateOne. Expected: ({filter}, {update})",
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Process filter with MongoDB syntax
		filterStr := params[0]
		updateStr := params[1]

		// Process the filter to handle MongoDB syntax
		var filter bson.M
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", filterStr)

			// Process the query parameters to handle MongoDB syntax
			jsonFilterStr, err := processMongoDBQueryParams(filterStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted filter: %s", jsonFilterStr)

			if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Process update with MongoDB syntax
		var update bson.M
		if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB update: %s", updateStr)

			// Process the query parameters to handle MongoDB syntax
			jsonUpdateStr, err := processMongoDBQueryParams(updateStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process update parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted update: %s", jsonUpdateStr)

			if err := json.Unmarshal([]byte(jsonUpdateStr), &update); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse update after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the updateOne operation
		updateResult, err := collection.UpdateOne(ctx, filter, update)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute updateOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// Check if any document was matched
		if updateResult.MatchedCount == 0 {
			log.Printf("MongoDBTransaction -> ExecuteQuery -> No document matched the filter criteria for updateOne")
		}

		result = map[string]interface{}{
			"matchedCount":  updateResult.MatchedCount,
			"modifiedCount": updateResult.ModifiedCount,
			"upsertedId":    updateResult.UpsertedID,
		}

	case "updateMany":
		// Parse the parameters as filter and update
		// Expected format: ({filter}, {update})
		params := strings.SplitN(paramsStr, ",", 2)
		if len(params) != 2 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: "Invalid parameters for updateMany. Expected: ({filter}, {update})",
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Process filter with MongoDB syntax
		filterStr := params[0]
		updateStr := params[1]

		// Process the filter to handle MongoDB syntax
		var filter bson.M
		if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", filterStr)

			// Process the query parameters to handle MongoDB syntax
			jsonFilterStr, err := processMongoDBQueryParams(filterStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted filter: %s", jsonFilterStr)

			if err := json.Unmarshal([]byte(jsonFilterStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Process update with MongoDB syntax
		var update bson.M
		if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
			// Try to handle MongoDB syntax with unquoted keys
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB update: %s", updateStr)

			// Process the query parameters to handle MongoDB syntax
			jsonUpdateStr, err := processMongoDBQueryParams(updateStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process update parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted update: %s", jsonUpdateStr)

			if err := json.Unmarshal([]byte(jsonUpdateStr), &update); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse update after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}
		}

		// Execute the updateMany operation
		updateResult, err := collection.UpdateMany(ctx, filter, update)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute updateMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"matchedCount":  updateResult.MatchedCount,
			"modifiedCount": updateResult.ModifiedCount,
			"upsertedId":    updateResult.UpsertedID,
		}

	case "deleteOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB query: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process query parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted query: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse query parameters after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after ObjectId conversion: %s", string(filterJSON))
		}

		// Execute the deleteOne operation
		deleteResult, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// Check if any document was deleted
		if deleteResult.DeletedCount == 0 {
			log.Printf("MongoDBTransaction -> ExecuteQuery -> No document matched the filter criteria for deleteOne")
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "deleteMany":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and operators like $or
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax

			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted filter: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse filter after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Handle ObjectId in the filter
			if err := processObjectIds(filter); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Log the final filter for debugging
			filterJSON, _ := json.Marshal(filter)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final filter after conversion: %s", string(filterJSON))
		}

		// Execute the deleteMany operation
		deleteResult, err := collection.DeleteMany(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteMany operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "aggregate":
		// Parse the parameters as a pipeline
		var pipeline []bson.M
		if err := json.Unmarshal([]byte(paramsStr), &pipeline); err != nil {
			// Try to handle MongoDB syntax with unquoted keys and ObjectId
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB aggregation pipeline: %s", paramsStr)

			// Process the query parameters to handle MongoDB syntax
			jsonStr, err := processMongoDBQueryParams(paramsStr)
			if err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to process aggregation pipeline: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted aggregation pipeline: %s", jsonStr)

			if err := json.Unmarshal([]byte(jsonStr), &pipeline); err != nil {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to parse aggregation pipeline after conversion: %v", err),
						Code:    "INVALID_PARAMETERS",
					},
				}
			}

			// Process ObjectIds in each stage of the pipeline
			for _, stage := range pipeline {
				if err := processObjectIds(stage); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds in aggregation pipeline: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}

			// Log the final pipeline for debugging
			pipelineJSON, _ := json.Marshal(pipeline)
			log.Printf("MongoDBTransaction -> ExecuteQuery -> Final aggregation pipeline after ObjectId conversion: %s", string(pipelineJSON))
		}

		// Convert []bson.M to mongo.Pipeline
		mongoPipeline := make(mongo.Pipeline, len(pipeline))
		for i, stage := range pipeline {
			// Convert each stage to bson.D
			stageD := bson.D{}
			for k, v := range stage {
				stageD = append(stageD, bson.E{Key: k, Value: v})
			}
			mongoPipeline[i] = stageD
		}

		// Execute the aggregate operation
		cursor, err := collection.Aggregate(ctx, mongoPipeline)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute aggregate operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}
		defer cursor.Close(ctx)

		// Decode the results
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to decode aggregation results: %v", err),
					Code:    "DECODE_ERROR",
				},
			}
		}

		result = results

	case "countDocuments":
		// Parse the parameters as a BSON filter
		var filter bson.M

		// Handle empty parameters for countDocuments()
		if strings.TrimSpace(paramsStr) == "" {
			// Use an empty filter to count all documents
			filter = bson.M{}
		} else {
			// Parse the provided filter
			if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
				// Try to handle MongoDB syntax with unquoted keys
				log.Printf("MongoDBTransaction -> ExecuteQuery -> Attempting to parse MongoDB filter: %s", paramsStr)

				// Process the query parameters to handle MongoDB syntax
				jsonStr, err := processMongoDBQueryParams(paramsStr)
				if err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process filter parameters: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				log.Printf("MongoDBTransaction -> ExecuteQuery -> Converted filter: %s", jsonStr)

				if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to parse filter: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}

				// Handle ObjectId in the filter
				if err := processObjectIds(filter); err != nil {
					return &QueryExecutionResult{
						Error: &dtos.QueryError{
							Message: fmt.Sprintf("Failed to process ObjectIds: %v", err),
							Code:    "INVALID_PARAMETERS",
						},
					}
				}
			}
		}

		// Execute the countDocuments operation
		count, err := collection.CountDocuments(ctx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute countDocuments operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"count": count,
		}

	case "createCollection":
		// Execute the createCollection operation with default options
		// We're simplifying this implementation to avoid complex option handling
		err := collection.Database().CreateCollection(ctx, collectionName)
		if err != nil {
			// Check if collection already exists
			if strings.Contains(err.Error(), "already exists") {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Collection '%s' already exists", collectionName),
						Code:    "COLLECTION_EXISTS",
					},
				}
			}

			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to create collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' created successfully", collectionName),
		}

	case "dropCollection":
		// Execute the dropCollection operation
		err := collection.Drop(ctx)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to drop collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' dropped successfully", collectionName),
		}

	case "drop":
		// Check if collection exists before dropping
		collections, err := tx.Wrapper.Client.Database(tx.Wrapper.Database).ListCollectionNames(ctx, bson.M{"name": collectionName})
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to check if collection exists: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		// If collection doesn't exist, return an error
		if len(collections) == 0 {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Collection '%s' does not exist", collectionName),
					Code:    "COLLECTION_NOT_FOUND",
				},
			}
		}

		// Execute the drop operation
		err = collection.Drop(ctx)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to drop collection: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"ok":      1,
			"message": fmt.Sprintf("Collection '%s' dropped successfully", collectionName),
		}

	default:
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Unsupported MongoDB operation: %s", operation),
				Code:    "UNSUPPORTED_OPERATION",
			},
		}
	}

	// Convert the result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Failed to marshal result to JSON: %v", err),
				Code:    "JSON_ERROR",
			},
		}
	}

	var resultMap map[string]interface{}
	if tempResultMap, ok := result.(map[string]interface{}); ok {
		// Create a result map
		resultMap = tempResultMap
	} else {
		resultMap = map[string]interface{}{
			"results": result,
		}
	}

	executionTime := int(time.Since(startTime).Milliseconds())
	log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in %d ms", executionTime)

	return &QueryExecutionResult{
		Result:        resultMap,
		ResultJSON:    string(resultJSON),
		ExecutionTime: executionTime,
	}
}

// processMongoDBQueryParams processes MongoDB query parameters
func processMongoDBQueryParams(paramsStr string) (string, error) {
	// Log the original string for debugging
	log.Printf("Original MongoDB query params: %s", paramsStr)

	// Extract modifiers from the query string
	var modifiersStr string
	if idx := strings.Index(paramsStr, "})."); idx != -1 {
		// Save the modifiers part for later processing
		modifiersStr = paramsStr[idx+1:]
		// Only keep the filter part
		paramsStr = paramsStr[:idx+1]
		log.Printf("Extracted filter part: %s", paramsStr)
		log.Printf("Extracted modifiers part: %s", modifiersStr)
	}

	// Check for offset_size in skip() - this is a special case for pagination
	// offset_size is a placeholder that will be replaced with the actual offset value
	// by the chat service when executing paginated queries.
	// For example, db.posts.find({}).skip(offset_size).limit(50) will become
	// db.posts.find({}).skip(50).limit(50) when requesting the second page with page size 50.
	// This replacement happens in the chat_service.go GetQueryResults function.
	if modifiersStr != "" {
		skipRegex := regexp.MustCompile(`\.skip\(offset_size\)`)
		if skipRegex.MatchString(modifiersStr) {
			log.Printf("Found offset_size in skip(), this will be replaced by the actual offset value")
		}
	}

	// First, handle MongoDB operators at the beginning of objects
	// For example: {$or: [...]} -> {"$or": [...]}
	operatorPattern := regexp.MustCompile(`\{\s*(\$[a-zA-Z]+):\s*`)
	paramsStr = operatorPattern.ReplaceAllString(paramsStr, `{"$1": `)

	// Handle ObjectId syntax: ObjectId('...') -> {"$oid":"..."}
	// This pattern matches both ObjectId('...') and ObjectId("...")
	objectIdPattern := regexp.MustCompile(`ObjectId\(['"]([^'"]+)['"]\)`)
	paramsStr = objectIdPattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the ObjectId value
		re := regexp.MustCompile(`ObjectId\(['"]([^'"]+)['"]\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// Return the proper JSON format for ObjectId
		return fmt.Sprintf(`{"$oid":"%s"}`, matches[1])
	})

	// Handle new Date() syntax with various formats:
	// 1. new Date() without parameters -> current date in ISO format
	// 2. new Date("...") or new Date('...') with quoted date string
	// 3. new Date(year, month, day, ...) with numeric parameters

	// First, handle new Date() without parameters
	emptyDatePattern := regexp.MustCompile(`new\s+Date\(\s*\)`)
	paramsStr = emptyDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Return current date in ISO format
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Handle new Date("...") and new Date('...') with quoted date string
	quotedDatePattern := regexp.MustCompile(`new\s+Date\(['"]([^'"]+)['"]\)`)
	paramsStr = quotedDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// Extract the date value
		re := regexp.MustCompile(`new\s+Date\(['"]([^'"]+)['"]\)`)
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		// Return the proper JSON format for Date
		return fmt.Sprintf(`{"$date":"%s"}`, matches[1])
	})

	// Handle new Date(year, month, day, ...) with numeric parameters
	// This is more complex and would require parsing the parameters and constructing a date
	// For now, we'll replace it with the current date as a fallback
	numericDatePattern := regexp.MustCompile(`new\s+Date\(([^'")\s][^)]*)\)`)
	paramsStr = numericDatePattern.ReplaceAllStringFunc(paramsStr, func(match string) string {
		// For now, return current date in ISO format as a fallback
		// In a more complete implementation, we would parse the numeric parameters
		return fmt.Sprintf(`{"$date":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// Log the processed string for debugging
	log.Printf("After ObjectId and Date replacement: %s", paramsStr)

	// Temporarily replace $oid and $date with placeholders to prevent them from being modified
	paramsStr = strings.ReplaceAll(paramsStr, "$oid", "__MONGODB_OID__")
	paramsStr = strings.ReplaceAll(paramsStr, "$date", "__MONGODB_DATE__")

	// Handle field names that might not be quoted
	// This regex matches field names followed by a colon, ensuring they're properly quoted
	fieldNameRegex := regexp.MustCompile(`([,{])\s*(\w+)\s*:`)
	paramsStr = fieldNameRegex.ReplaceAllString(paramsStr, `$1"$2":`)

	// Handle single quotes for string values
	// Use a standard approach instead of negative lookbehind which isn't supported in Go
	singleQuoteRegex := regexp.MustCompile(`'([^']*)'`)
	paramsStr = singleQuoteRegex.ReplaceAllString(paramsStr, `"$1"`)

	// Restore placeholders
	paramsStr = strings.ReplaceAll(paramsStr, "__MONGODB_OID__", "$oid")
	paramsStr = strings.ReplaceAll(paramsStr, "__MONGODB_DATE__", "$date")

	// Ensure the document is valid JSON
	// Check if it's an object and add missing quotes to field names
	if strings.HasPrefix(paramsStr, "{") && strings.HasSuffix(paramsStr, "}") {
		// Add quotes to any remaining unquoted field names
		// This regex matches field names that aren't already quoted
		unquotedFieldRegex := regexp.MustCompile(`([,{])\s*(\w+)\s*:`)
		for unquotedFieldRegex.MatchString(paramsStr) {
			paramsStr = unquotedFieldRegex.ReplaceAllString(paramsStr, `$1"$2":`)
		}
	}

	// Log the final processed string for debugging
	log.Printf("Final processed MongoDB query params: %s", paramsStr)

	return paramsStr, nil
}

// processObjectIds processes ObjectId syntax in MongoDB queries
func processObjectIds(filter map[string]interface{}) error {
	// Log the input filter for debugging
	filterJSON, _ := json.Marshal(filter)
	log.Printf("processObjectIds input: %s", string(filterJSON))

	for key, value := range filter {
		switch v := value.(type) {
		case map[string]interface{}:
			// Check if this is an ObjectId
			if oidStr, ok := v["$oid"].(string); ok && len(v) == 1 {
				// Convert to ObjectID
				oid, err := primitive.ObjectIDFromHex(oidStr)
				if err != nil {
					return fmt.Errorf("invalid ObjectId: %v", err)
				}
				filter[key] = oid
				log.Printf("Converted ObjectId %s to %v", oidStr, oid)
			} else if dateStr, ok := v["$date"].(string); ok && len(v) == 1 {
				// Convert to time.Time
				date, err := time.Parse(time.RFC3339, dateStr)
				if err != nil {
					// Try other common date formats
					formats := []string{
						time.RFC3339,
						"2006-01-02T15:04:05Z",
						"2006-01-02",
						"2006/01/02",
						"01/02/2006",
						"01-02-2006",
						time.ANSIC,
						time.UnixDate,
						time.RubyDate,
						time.RFC822,
						time.RFC822Z,
						time.RFC850,
						time.RFC1123,
						time.RFC1123Z,
					}

					parsed := false
					for _, format := range formats {
						if parsedDate, parseErr := time.Parse(format, dateStr); parseErr == nil {
							date = parsedDate
							parsed = true
							break
						}
					}

					if !parsed {
						return fmt.Errorf("invalid date format: %v", err)
					}
				}
				filter[key] = date
				log.Printf("Converted date %s to %v", dateStr, date)
			} else {
				// Recursively process nested maps
				if err := processObjectIds(v); err != nil {
					return err
				}
			}
		case []interface{}:
			// Process arrays
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if err := processObjectIds(itemMap); err != nil {
						return err
					}
					v[i] = itemMap
				}
			}
		}
	}

	// Log the output filter for debugging
	filterJSON, _ = json.Marshal(filter)
	log.Printf("processObjectIds output (after ObjectId and Date conversion): %s", string(filterJSON))

	return nil
}

// Add this new function to extract modifiers from the query string
// Add this after the processObjectIds function
func extractModifiers(query string) struct {
	Skip       int64
	Limit      int64
	Sort       string
	Projection string
	Count      bool
} {
	modifiers := struct {
		Skip       int64
		Limit      int64
		Sort       string
		Projection string
		Count      bool
	}{}

	// Check if the query string is empty or doesn't contain any modifiers
	if query == "" || !strings.Contains(query, ".") {
		return modifiers
	}

	// Extract skip
	skipRegex := regexp.MustCompile(`\.skip\((\d+)\)`)
	skipMatches := skipRegex.FindStringSubmatch(query)
	if len(skipMatches) > 1 {
		skip, err := strconv.ParseInt(skipMatches[1], 10, 64)
		if err == nil {
			modifiers.Skip = skip
		}
	}

	// Extract limit
	limitRegex := regexp.MustCompile(`\.limit\((\d+)\)`)
	limitMatches := limitRegex.FindStringSubmatch(query)
	if len(limitMatches) > 1 {
		limit, err := strconv.ParseInt(limitMatches[1], 10, 64)
		if err == nil {
			modifiers.Limit = limit
		}
	}

	// Extract count
	countRegex := regexp.MustCompile(`\.count\(\s*\)`)
	countMatches := countRegex.FindStringSubmatch(query)
	if len(countMatches) > 0 {
		modifiers.Count = true
		log.Printf("extractModifiers -> Detected count() modifier")
	}

	// Extract projection
	projectionRegex := regexp.MustCompile(`\.project\(([^)]+)\)`)
	projectionMatches := projectionRegex.FindStringSubmatch(query)
	if len(projectionMatches) > 1 {
		// Get the raw projection expression
		projectionExpr := projectionMatches[1]
		modifiers.Projection = projectionExpr
		log.Printf("extractModifiers -> Extracted projection expression: %s", modifiers.Projection)
	}

	// Extract sort - improved to handle complex sort expressions
	sortRegex := regexp.MustCompile(`\.sort\(([^)]+)\)`)
	sortMatches := sortRegex.FindStringSubmatch(query)
	if len(sortMatches) > 1 {
		// Get the raw sort expression
		sortExpr := sortMatches[1]

		// Check if it's a valid JSON object already
		if strings.HasPrefix(sortExpr, "{") && strings.HasSuffix(sortExpr, "}") {
			modifiers.Sort = sortExpr
		} else {
			// Try to convert to a valid JSON object
			// This handles cases like "field" or field without quotes
			sortExpr = strings.Trim(sortExpr, "\"' ")
			if !strings.HasPrefix(sortExpr, "{") {
				// Simple field name, default to ascending order
				modifiers.Sort = fmt.Sprintf(`{"%s": 1}`, sortExpr)
			} else {
				// It's already an object-like expression
				modifiers.Sort = sortExpr
			}
		}

		log.Printf("extractModifiers -> Extracted sort expression: %s", modifiers.Sort)
	}

	return modifiers
}

// SafeBeginTx is a helper function to safely begin a transaction with proper error handling
func (d *MongoDBDriver) SafeBeginTx(ctx context.Context, conn *Connection) (Transaction, error) {
	log.Printf("MongoDBDriver -> SafeBeginTx -> Safely beginning MongoDB transaction")

	tx := d.BeginTx(ctx, conn)

	// Check if the transaction has an error
	if mongoTx, ok := tx.(*MongoDBTransaction); ok && mongoTx.Error != nil {
		log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction creation failed: %v", mongoTx.Error)
		return nil, mongoTx.Error
	}

	// Check if the transaction has a nil session
	if mongoTx, ok := tx.(*MongoDBTransaction); ok && mongoTx.Session == nil {
		log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction has nil session")
		return nil, fmt.Errorf("transaction has nil session")
	}

	log.Printf("MongoDBDriver -> SafeBeginTx -> Transaction created successfully")
	return tx, nil
}
