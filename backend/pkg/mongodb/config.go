package mongodb

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBClient struct {
	Client *mongo.Client
	Config MongoDbConfigModel
}

func InitializeDatabaseConnection(config MongoDbConfigModel) *MongoDBClient {
	// Replace with your MongoDB Atlas connection string
	connectionURI := config.ConnectionUrl

	clientOptions := options.Client().ApplyURI(connectionURI)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	mongoClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("MongoDB Connection Error: %v", err)
	}

	// Ping the database to verify connection
	err = mongoClient.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("MongoDB Ping Error: %v", err)
	}

	log.Println("✨ Connected to MongoDB.")

	return &MongoDBClient{
		Client: mongoClient,
		Config: config,
	}
}

func (client *MongoDBClient) GetCollectionByName(collectionName string) *mongo.Collection {
	collection := client.Client.Database(client.Config.DatabaseName).Collection(collectionName)
	return collection
}
