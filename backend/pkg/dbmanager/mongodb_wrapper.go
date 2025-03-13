package dbmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBExecutor implements the DBExecutor interface for MongoDB
type MongoDBExecutor struct {
	wrapper *MongoDBWrapper
	conn    *Connection
}

// NewMongoDBExecutor creates a new MongoDB executor
func NewMongoDBExecutor(conn *Connection) (*MongoDBExecutor, error) {
	wrapper, ok := conn.MongoDBObj.(*MongoDBWrapper)
	if !ok {
		return nil, fmt.Errorf("invalid MongoDB connection")
	}

	return &MongoDBExecutor{
		wrapper: wrapper,
		conn:    conn,
	}, nil
}

// GetDB returns nil for MongoDB as it doesn't use GORM
func (e *MongoDBExecutor) GetDB() *sql.DB {
	return nil // MongoDB doesn't use sql.DB
}

// GetMongoDatabase returns the MongoDB database
func (e *MongoDBExecutor) GetMongoDatabase() *mongo.Database {
	if e.wrapper == nil {
		return nil
	}
	return e.wrapper.Client.Database(e.wrapper.Database)
}

// GetConnection returns the underlying connection
func (e *MongoDBExecutor) GetConnection() *Connection {
	return e.conn
}

// ExecuteQuery executes a MongoDB query
func (e *MongoDBExecutor) ExecuteQuery(query string) (*QueryExecutionResult, error) {
	ctx := context.Background()
	driver := &MongoDBDriver{}
	return driver.ExecuteQuery(ctx, e.conn, query, ""), nil
}

// QueryRows executes a MongoDB query and unmarshals the results into the provided destination
func (e *MongoDBExecutor) QueryRows(query string, dest *[]map[string]interface{}, values ...interface{}) error {
	// Parse the MongoDB query
	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "db") {
		return fmt.Errorf("invalid MongoDB query format. Expected: db.collection.operation({...})")
	}

	collectionName := parts[1]
	operationWithParams := parts[2]

	// Split the operation and parameters
	openParenIndex := strings.Index(operationWithParams, "(")
	closeParenIndex := strings.LastIndex(operationWithParams, ")")

	if openParenIndex == -1 || closeParenIndex == -1 || closeParenIndex <= openParenIndex {
		return fmt.Errorf("invalid MongoDB query format. Expected: operation({...})")
	}

	operation := operationWithParams[:openParenIndex]
	paramsStr := operationWithParams[openParenIndex+1 : closeParenIndex]

	// Get the MongoDB collection
	collection := e.wrapper.Client.Database(e.wrapper.Database).Collection(collectionName)

	// Execute the query based on the operation
	ctx := context.Background()
	switch operation {
	case "find":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return fmt.Errorf("failed to parse query parameters: %v", err)
		}

		// Execute the find operation
		cursor, err := collection.Find(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to execute find operation: %v", err)
		}
		defer cursor.Close(ctx)

		// Decode the results
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return fmt.Errorf("failed to decode results: %v", err)
		}

		// Convert bson.M to map[string]interface{}
		*dest = make([]map[string]interface{}, len(results))
		for i, result := range results {
			(*dest)[i] = make(map[string]interface{})
			for k, v := range result {
				(*dest)[i][k] = convertMongoDBValue(v)
			}
		}

	case "findOne":
		// Parse the parameters as a BSON filter
		var filter bson.M
		if err := json.Unmarshal([]byte(paramsStr), &filter); err != nil {
			return fmt.Errorf("failed to parse query parameters: %v", err)
		}

		// Execute the findOne operation
		result := collection.FindOne(ctx, filter)
		if result.Err() != nil {
			if result.Err() == mongo.ErrNoDocuments {
				// No documents found, set dest to empty slice
				*dest = []map[string]interface{}{}
				return nil
			}
			return fmt.Errorf("failed to execute findOne operation: %v", result.Err())
		}

		// Decode the result
		var doc bson.M
		if err := result.Decode(&doc); err != nil {
			return fmt.Errorf("failed to decode result: %v", err)
		}

		// Convert bson.M to map[string]interface{}
		*dest = make([]map[string]interface{}, 1)
		(*dest)[0] = make(map[string]interface{})
		for k, v := range doc {
			(*dest)[0][k] = convertMongoDBValue(v)
		}

	case "aggregate":
		// Parse the parameters as a pipeline
		var pipeline []bson.M
		if err := json.Unmarshal([]byte(paramsStr), &pipeline); err != nil {
			return fmt.Errorf("failed to parse aggregation pipeline: %v", err)
		}

		// Convert []bson.M to mongo.Pipeline
		mongoPipeline := make(mongo.Pipeline, len(pipeline))
		for i, stage := range pipeline {
			mongoPipeline[i] = bson.D{{Key: "$match", Value: stage}}
		}

		// Execute the aggregate operation
		cursor, err := collection.Aggregate(ctx, mongoPipeline)
		if err != nil {
			return fmt.Errorf("failed to execute aggregate operation: %v", err)
		}
		defer cursor.Close(ctx)

		// Decode the results
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return fmt.Errorf("failed to decode aggregation results: %v", err)
		}

		// Convert bson.M to map[string]interface{}
		*dest = make([]map[string]interface{}, len(results))
		for i, result := range results {
			(*dest)[i] = make(map[string]interface{})
			for k, v := range result {
				(*dest)[i][k] = convertMongoDBValue(v)
			}
		}

	default:
		return fmt.Errorf("unsupported MongoDB operation for QueryRows: %s", operation)
	}

	return nil
}

// convertMongoDBValue converts MongoDB-specific types to JSON-friendly formats
func convertMongoDBValue(value interface{}) interface{} {
	switch v := value.(type) {
	case primitive.ObjectID:
		return v.Hex()
	case primitive.DateTime:
		return time.Unix(0, int64(v)*int64(time.Millisecond)).Format(time.RFC3339)
	case primitive.A:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertMongoDBValue(item)
		}
		return result
	case bson.M:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = convertMongoDBValue(val)
		}
		return result
	case bson.D:
		result := make(map[string]interface{})
		for _, elem := range v {
			result[elem.Key] = convertMongoDBValue(elem.Value)
		}
		return result
	case primitive.Binary:
		return fmt.Sprintf("Binary(%d bytes)", len(v.Data))
	default:
		return v
	}
}

// ListCollections lists all collections in the MongoDB database
func (e *MongoDBExecutor) ListCollections(ctx context.Context) ([]string, error) {
	log.Printf("MongoDBExecutor -> ListCollections -> Listing collections in database %s", e.wrapper.Database)
	collections, err := e.wrapper.Client.Database(e.wrapper.Database).ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %v", err)
	}
	log.Printf("MongoDBExecutor -> ListCollections -> Found %d collections: %v", len(collections), collections)
	return collections, nil
}

// SampleCollection samples documents from a MongoDB collection
func (e *MongoDBExecutor) SampleCollection(ctx context.Context, collectionName string, sampleSize int) ([]bson.M, error) {
	log.Printf("MongoDBExecutor -> SampleCollection -> Sampling collection %s with sample size %d", collectionName, sampleSize)

	// First, check if the collection has any documents
	count, err := e.wrapper.Client.Database(e.wrapper.Database).Collection(collectionName).CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Printf("MongoDBExecutor -> SampleCollection -> Error counting documents in collection %s: %v", collectionName, err)
		return nil, fmt.Errorf("failed to count documents: %v", err)
	}

	log.Printf("MongoDBExecutor -> SampleCollection -> Collection %s has %d documents", collectionName, count)

	// If collection is empty, return empty result
	if count == 0 {
		log.Printf("MongoDBExecutor -> SampleCollection -> Collection %s is empty, returning empty result", collectionName)
		return []bson.M{}, nil
	}

	// Ensure sample size is reasonable
	if sampleSize <= 0 {
		sampleSize = 10
	} else if sampleSize > 1000 {
		sampleSize = 1000
	}

	// If sample size is greater than document count, adjust it
	if int64(sampleSize) > count {
		sampleSize = int(count)
		log.Printf("MongoDBExecutor -> SampleCollection -> Adjusted sample size to %d to match document count", sampleSize)
	}

	// Try two approaches: first with $sample, then with find if that fails

	// Approach 1: Use the $sample aggregation stage to get random documents
	pipeline := mongo.Pipeline{
		{{Key: "$sample", Value: bson.M{"size": sampleSize}}},
	}

	cursor, err := e.wrapper.Client.Database(e.wrapper.Database).Collection(collectionName).Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("MongoDBExecutor -> SampleCollection -> Error using $sample aggregation: %v, falling back to find()", err)
		// Fall back to find() if aggregation fails
		findCursor, findErr := e.wrapper.Client.Database(e.wrapper.Database).Collection(collectionName).Find(ctx, bson.M{})
		if findErr != nil {
			log.Printf("MongoDBExecutor -> SampleCollection -> Error using find(): %v", findErr)
			return nil, fmt.Errorf("failed to query collection: %v", findErr)
		}
		defer findCursor.Close(ctx)

		var results []bson.M
		if err := findCursor.All(ctx, &results); err != nil {
			log.Printf("MongoDBExecutor -> SampleCollection -> Error decoding find results: %v", err)
			return nil, fmt.Errorf("failed to decode find results: %v", err)
		}

		// Limit results to sample size
		if len(results) > sampleSize {
			results = results[:sampleSize]
		}

		log.Printf("MongoDBExecutor -> SampleCollection -> Retrieved %d documents using find() from collection %s", len(results), collectionName)
		return results, nil
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("MongoDBExecutor -> SampleCollection -> Error decoding sample results: %v", err)
		return nil, fmt.Errorf("failed to decode sample results: %v", err)
	}

	log.Printf("MongoDBExecutor -> SampleCollection -> Sampled %d documents from collection %s", len(results), collectionName)
	return results, nil
}

// ExecuteRawCommand executes a raw MongoDB command
func (e *MongoDBExecutor) ExecuteRawCommand(ctx context.Context, command interface{}) (bson.M, error) {
	var result bson.M
	err := e.wrapper.Client.Database(e.wrapper.Database).RunCommand(ctx, command).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to execute raw command: %v", err)
	}
	return result, nil
}

// CreateCollection creates a new MongoDB collection
func (e *MongoDBExecutor) CreateCollection(ctx context.Context, name string, options *options.CreateCollectionOptions) error {
	err := e.wrapper.Client.Database(e.wrapper.Database).CreateCollection(ctx, name, options)
	if err != nil {
		return fmt.Errorf("failed to create collection: %v", err)
	}
	return nil
}

// DropCollection drops a MongoDB collection
func (e *MongoDBExecutor) DropCollection(ctx context.Context, name string) error {
	err := e.wrapper.Client.Database(e.wrapper.Database).Collection(name).Drop(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop collection: %v", err)
	}
	return nil
}

// CreateIndex creates an index on a MongoDB collection
func (e *MongoDBExecutor) CreateIndex(ctx context.Context, collectionName string, keys bson.D, options *options.IndexOptions) (string, error) {
	indexModel := mongo.IndexModel{
		Keys:    keys,
		Options: options,
	}

	indexName, err := e.wrapper.Client.Database(e.wrapper.Database).Collection(collectionName).Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return "", fmt.Errorf("failed to create index: %v", err)
	}
	return indexName, nil
}

// DropIndex drops an index from a MongoDB collection
func (e *MongoDBExecutor) DropIndex(ctx context.Context, collectionName string, indexName string) error {
	_, err := e.wrapper.Client.Database(e.wrapper.Database).Collection(collectionName).Indexes().DropOne(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to drop index: %v", err)
	}
	return nil
}

// ParseMongoDBQuery parses a MongoDB query string into its components
func (e *MongoDBExecutor) ParseMongoDBQuery(query string) (string, string, string, error) {
	// Parse the MongoDB query
	parts := strings.SplitN(query, ".", 3)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "db") {
		return "", "", "", fmt.Errorf("invalid MongoDB query format. Expected: db.collection.operation({...})")
	}

	collectionName := parts[1]
	operationWithParams := parts[2]

	// Split the operation and parameters
	openParenIndex := strings.Index(operationWithParams, "(")
	closeParenIndex := strings.LastIndex(operationWithParams, ")")

	if openParenIndex == -1 || closeParenIndex == -1 || closeParenIndex <= openParenIndex {
		return "", "", "", fmt.Errorf("invalid MongoDB query format. Expected: operation({...})")
	}

	operation := operationWithParams[:openParenIndex]
	paramsStr := operationWithParams[openParenIndex+1 : closeParenIndex]

	return collectionName, operation, paramsStr, nil
}

// GetCollectionStats gets statistics for a MongoDB collection
func (e *MongoDBExecutor) GetCollectionStats(ctx context.Context, collectionName string) (bson.M, error) {
	command := bson.D{{Key: "collStats", Value: collectionName}}
	return e.ExecuteRawCommand(ctx, command)
}

// GetDatabaseStats gets statistics for the MongoDB database
func (e *MongoDBExecutor) GetDatabaseStats(ctx context.Context) (bson.M, error) {
	command := bson.D{{Key: "dbStats", Value: 1}}
	return e.ExecuteRawCommand(ctx, command)
}

// Close closes the MongoDB connection
func (e *MongoDBExecutor) Close() error {
	// Connection is managed by the MongoDB driver
	return nil
}

// Exec executes a MongoDB command
func (e *MongoDBExecutor) Exec(command string, values ...interface{}) error {
	// Parse and execute the MongoDB command
	log.Printf("MongoDBExecutor -> Exec -> Command: %s", command)

	// Execute the command using the MongoDB driver
	ctx := context.Background()
	driver := &MongoDBDriver{}
	result := driver.ExecuteQuery(ctx, e.conn, command, "")

	// Check for errors
	if result.Error != nil {
		return fmt.Errorf("failed to execute MongoDB command: %v", result.Error.Message)
	}

	return nil
}

// Raw executes a raw MongoDB command
func (e *MongoDBExecutor) Raw(command string, values ...interface{}) error {
	// Parse and execute the MongoDB command
	log.Printf("MongoDBExecutor -> Raw -> Command: %s", command)

	// Execute the command using the MongoDB driver
	ctx := context.Background()
	driver := &MongoDBDriver{}
	result := driver.ExecuteQuery(ctx, e.conn, command, "")

	// Check for errors
	if result.Error != nil {
		return fmt.Errorf("failed to execute MongoDB command: %v", result.Error.Message)
	}

	return nil
}

// Query executes a MongoDB query and scans the result into dest
func (e *MongoDBExecutor) Query(query string, dest interface{}, values ...interface{}) error {
	// Parse and execute the MongoDB query
	log.Printf("MongoDBExecutor -> Query -> Query: %s", query)

	// Convert dest to the expected type
	destMap, ok := dest.(*[]map[string]interface{})
	if !ok {
		return fmt.Errorf("destination must be *[]map[string]interface{}")
	}

	// Execute the query using QueryRows
	return e.QueryRows(query, destMap, values...)
}

// GetSchema fetches the MongoDB schema
func (e *MongoDBExecutor) GetSchema(ctx context.Context) (*SchemaInfo, error) {
	// Get the schema fetcher for MongoDB
	driver := &MongoDBDriver{}

	// Default to ALL collections
	selectedCollections := []string{"ALL"}

	// Get the schema
	return driver.GetSchema(ctx, e, selectedCollections)
}

// GetTableChecksum calculates a checksum for a MongoDB collection
func (e *MongoDBExecutor) GetTableChecksum(ctx context.Context, table string) (string, error) {
	// Use the MongoDB driver to get the table checksum
	driver := &MongoDBDriver{}
	return driver.GetTableChecksum(ctx, e, table)
}
