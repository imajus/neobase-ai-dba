package dbmanager

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoDBWrapper wraps a MongoDB client
type MongoDBWrapper struct {
	Client   *mongo.Client
	Database string
}

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
