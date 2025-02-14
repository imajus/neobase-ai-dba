package di

import (
	"log"
	"neobase-ai/config"
	"neobase-ai/pkg/mongodb"
	"neobase-ai/pkg/redis"

	"go.uber.org/dig"
)

var DiContainer *dig.Container

func Initialize() {
	DiContainer = dig.New()

	dbConfig := mongodb.MongoDbConfigModel{
		ConnectionUrl: config.Env.MongoURI,
		DatabaseName:  config.Env.MongoDatabaseName,
	}
	mongodbClient := mongodb.InitializeDatabaseConnection(dbConfig)
	redisClient, err := redis.RedisClient(config.Env.RedisHost, config.Env.RedisPort, config.Env.RedisUsername, config.Env.RedisPassword)
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}
	redisRepo := redis.NewRedisRepositories(redisClient)

	// Provide dependencies
	DiContainer.Provide(redisRepo)
	DiContainer.Provide(mongodbClient)
}
