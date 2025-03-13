package dbmanager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/utils"
	"net/url"
	"os"
	"strings"
	"time"

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
	opts := options.Find().SetLimit(100) // Sample up to 100 documents
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
	} else if limit > 10 {
		limit = 10 // Cap at 10 records to avoid large data transfers
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
	log.Printf("MongoDBDriver -> Connect -> Connecting to MongoDB at %s:%s", config.Host, config.Port)

	var uri string
	var tempFiles []string

	// Check if we're using SRV records (mongodb+srv://)
	isSRV := strings.Contains(config.Host, ".mongodb.net")
	protocol := "mongodb"
	if isSRV {
		protocol = "mongodb+srv"
		log.Printf("MongoDBDriver -> Connect -> MongoDB Atlas connection detected, using %s protocol", protocol)
	}

	// Base connection parameters
	if config.Username != nil && *config.Username != "" {
		// URL encode username and password to handle special characters
		encodedUsername := url.QueryEscape(*config.Username)
		encodedPassword := url.QueryEscape(*config.Password)

		// With authentication
		if isSRV {
			// For SRV records, don't include port
			uri = fmt.Sprintf("%s://%s:%s@%s/%s",
				protocol, encodedUsername, encodedPassword, config.Host, config.Database)
		} else {
			// Include port for standard connections
			uri = fmt.Sprintf("%s://%s:%s@%s:%s/%s",
				protocol, encodedUsername, encodedPassword, config.Host, *config.Port, config.Database)
		}
	} else {
		// Without authentication
		if isSRV {
			// For SRV records, don't include port
			uri = fmt.Sprintf("%s://%s/%s", protocol, config.Host, config.Database)
		} else {
			// Include port for standard connections
			uri = fmt.Sprintf("%s://%s:%s/%s", protocol, config.Host, *config.Port, config.Database)
		}
	}

	// Add connection options
	if isSRV {
		uri += "?retryWrites=true&w=majority"
	}

	// Configure SSL/TLS
	clientOptions := options.Client().ApplyURI(uri)
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
		// Disable SSL
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

	log.Printf("MongoDBDriver -> Connect -> Successfully connected to MongoDB at %s:%s", config.Host, config.Port)
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
func (d *MongoDBDriver) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string) *QueryExecutionResult {
	log.Printf("MongoDBDriver -> ExecuteQuery -> Executing MongoDB query: %s", query)
	startTime := time.Now()

	// Get the MongoDB wrapper from the connection
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB connection",
				Code:    "INVALID_CONNECTION",
			},
		}
	}

	// Parse the MongoDB query
	// MongoDB queries are expected in the format: db.collection.operation({...})
	// For example: db.users.find({name: "John"})
	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "db") {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: db.collection.operation({...})",
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

	operation := operationWithParams[:openParenIndex]
	paramsStr := operationWithParams[openParenIndex+1 : closeParenIndex]

	// Get the MongoDB collection
	collection := wrapper.Client.Database(wrapper.Database).Collection(collectionName)

	var result interface{}
	var err error

	log.Printf("MongoDBDriver -> ExecuteQuery -> operation: %s", operation)
	// Execute the operation based on the type
	switch operation {
	case "find":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse query parameters: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the find operation
		cursor, err := collection.Find(ctx, filter)
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
					Message: fmt.Sprintf("Failed to decode results: %v", err),
					Code:    "DECODE_ERROR",
				},
			}
		}

		result = results

	case "findOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse query parameters: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
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
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse document: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the insertOne operation
		insertResult, err := collection.InsertOne(ctx, document)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertOne operation: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"_id": insertResult.InsertedID,
		}

	case "insertMany":
		// Parse the parameters as an array of BSON documents
		var documents []interface{}
		if err := json.Unmarshal([]byte(paramsStr), &documents); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse documents: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
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
			"_ids":          insertResult.InsertedIDs,
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

		var filter bson.M
		var update bson.M
		if err := json.Unmarshal([]byte(params[0]), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}
		if err := json.Unmarshal([]byte(params[1]), &update); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse update: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
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

		var filter bson.M
		var update bson.M
		if err := json.Unmarshal([]byte(params[0]), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}
		if err := json.Unmarshal([]byte(params[1]), &update); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse update: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
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
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
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

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "deleteMany":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
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
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse aggregation pipeline: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Convert []bson.M to mongo.Pipeline
		mongoPipeline := make(mongo.Pipeline, len(pipeline))
		for i, stage := range pipeline {
			mongoPipeline[i] = bson.D{{Key: "$match", Value: stage}}
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
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
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

	// Create a result map
	resultMap := map[string]interface{}{
		"result": result,
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

	// Get the MongoDB wrapper from the connection
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return &MongoDBTransaction{
			Error: fmt.Errorf("invalid MongoDB connection"),
		}
	}

	// Start a new session
	session, err := wrapper.Client.StartSession()
	if err != nil {
		log.Printf("MongoDBDriver -> BeginTx -> Error starting MongoDB session: %v", err)
		return &MongoDBTransaction{
			Error: fmt.Errorf("failed to start MongoDB session: %v", err),
		}
	}

	// Start a transaction
	err = session.StartTransaction()
	if err != nil {
		log.Printf("MongoDBDriver -> BeginTx -> Error starting MongoDB transaction: %v", err)
		session.EndSession(ctx)
		return &MongoDBTransaction{
			Error: fmt.Errorf("failed to start MongoDB transaction: %v", err),
		}
	}

	// Create a new transaction object
	tx := &MongoDBTransaction{
		Session: session,
		Wrapper: wrapper,
		Error:   nil,
	}

	log.Printf("MongoDBDriver -> BeginTx -> MongoDB transaction started")
	return tx
}

// Commit commits a MongoDB transaction
func (tx *MongoDBTransaction) Commit() error {
	log.Printf("MongoDBTransaction -> Commit -> Committing MongoDB transaction")

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		return fmt.Errorf("cannot commit transaction with error: %v", tx.Error)
	}

	// Create a context with the session
	ctx := context.Background()
	sessionCtx := mongo.NewSessionContext(ctx, tx.Session)

	// Commit the transaction
	err := tx.Session.CommitTransaction(sessionCtx)
	if err != nil {
		return fmt.Errorf("failed to commit MongoDB transaction: %v", err)
	}

	// End the session
	tx.Session.EndSession(ctx)

	log.Printf("MongoDBTransaction -> Commit -> MongoDB transaction committed")
	return nil
}

// Rollback rolls back a MongoDB transaction
func (tx *MongoDBTransaction) Rollback() error {
	log.Printf("MongoDBTransaction -> Rollback -> Rolling back MongoDB transaction")

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		// If there was an error starting the transaction, just end the session
		tx.Session.EndSession(context.Background())
		return nil
	}

	// Create a context with the session
	ctx := context.Background()
	sessionCtx := mongo.NewSessionContext(ctx, tx.Session)

	// Abort the transaction
	err := tx.Session.AbortTransaction(sessionCtx)
	if err != nil {
		return fmt.Errorf("failed to abort MongoDB transaction: %v", err)
	}

	// End the session
	tx.Session.EndSession(ctx)

	log.Printf("MongoDBTransaction -> Rollback -> MongoDB transaction rolled back")
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
func (tx *MongoDBTransaction) ExecuteQuery(ctx context.Context, conn *Connection, query string, queryType string) *QueryExecutionResult {
	log.Printf("MongoDBTransaction -> ExecuteQuery -> Executing MongoDB query in transaction: %s", query)
	startTime := time.Now()

	// Check if there was an error starting the transaction
	if tx.Error != nil {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Transaction error: %v", tx.Error),
				Code:    "TRANSACTION_ERROR",
			},
		}
	}

	// Create a context with the session
	sessionCtx := mongo.NewSessionContext(ctx, tx.Session)

	// Parse the MongoDB query
	// MongoDB queries are expected in the format: db.collection.operation({...})
	// For example: db.users.find({name: "John"})
	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "db") {
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: "Invalid MongoDB query format. Expected: db.collection.operation({...})",
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

	operation := operationWithParams[:openParenIndex]
	paramsStr := operationWithParams[openParenIndex+1 : closeParenIndex]

	// Get the MongoDB collection
	collection := tx.Wrapper.Client.Database(tx.Wrapper.Database).Collection(collectionName)

	var result interface{}
	var err error

	log.Printf("MongoDBDriver -> ExecuteQuery -> operation: %s", operation)
	// Execute the operation based on the type
	switch operation {
	case "find":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse query parameters: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the find operation within the transaction
		cursor, err := collection.Find(sessionCtx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute find operation in transaction: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}
		defer cursor.Close(sessionCtx)

		// Decode the results
		var results []bson.M
		if err := cursor.All(sessionCtx, &results); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to decode results in transaction: %v", err),
					Code:    "DECODE_ERROR",
				},
			}
		}

		result = results

	case "findOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse query parameters: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the findOne operation within the transaction
		var doc bson.M
		err = collection.FindOne(sessionCtx, filter).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// No documents found, return empty result
				result = nil
			} else {
				return &QueryExecutionResult{
					Error: &dtos.QueryError{
						Message: fmt.Sprintf("Failed to execute findOne operation in transaction: %v", err),
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
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse document: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the insertOne operation within the transaction
		insertResult, err := collection.InsertOne(sessionCtx, document)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertOne operation in transaction: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"_ids": insertResult.InsertedID,
		}

	case "insertMany":
		// Parse the parameters as an array of BSON documents
		var documents []interface{}
		if err := json.Unmarshal([]byte(paramsStr), &documents); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse documents: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the insertMany operation within the transaction
		insertResult, err := collection.InsertMany(sessionCtx, documents)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute insertMany operation in transaction: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"_ids":          insertResult.InsertedIDs,
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

		var filter bson.M
		var update bson.M
		if err := json.Unmarshal([]byte(params[0]), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}
		if err := json.Unmarshal([]byte(params[1]), &update); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse update: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the updateOne operation within the transaction
		updateResult, err := collection.UpdateOne(sessionCtx, filter, update)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute updateOne operation in transaction: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"matchedCount":  updateResult.MatchedCount,
			"modifiedCount": updateResult.ModifiedCount,
			"_ids":          updateResult.UpsertedID,
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

		var filter bson.M
		var update bson.M
		if err := json.Unmarshal([]byte(params[0]), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}
		if err := json.Unmarshal([]byte(params[1]), &update); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse update: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the updateMany operation within the transaction
		updateResult, err := collection.UpdateMany(sessionCtx, filter, update)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute updateMany operation in transaction: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"matchedCount":  updateResult.MatchedCount,
			"modifiedCount": updateResult.ModifiedCount,
			"_ids":          updateResult.UpsertedID,
		}

	case "deleteOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the deleteOne operation within the transaction
		deleteResult, err := collection.DeleteOne(sessionCtx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteOne operation in transaction: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	case "deleteMany":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to parse filter: %v", err),
					Code:    "INVALID_PARAMETERS",
				},
			}
		}

		// Execute the deleteMany operation within the transaction
		deleteResult, err := collection.DeleteMany(sessionCtx, filter)
		if err != nil {
			return &QueryExecutionResult{
				Error: &dtos.QueryError{
					Message: fmt.Sprintf("Failed to execute deleteMany operation in transaction: %v", err),
					Code:    "EXECUTION_ERROR",
				},
			}
		}

		result = map[string]interface{}{
			"deletedCount": deleteResult.DeletedCount,
		}

	default:
		return &QueryExecutionResult{
			Error: &dtos.QueryError{
				Message: fmt.Sprintf("Unsupported MongoDB operation in transaction: %s", operation),
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

	// Create a result map
	resultMap := map[string]interface{}{
		"result": result,
	}

	executionTime := int(time.Since(startTime).Milliseconds())
	log.Printf("MongoDBTransaction -> ExecuteQuery -> MongoDB query executed in transaction in %d ms", executionTime)

	return &QueryExecutionResult{
		Result:        resultMap,
		ResultJSON:    string(resultJSON),
		ExecutionTime: executionTime,
	}
}
