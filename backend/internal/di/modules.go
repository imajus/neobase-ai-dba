package di

import (
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/handlers"
	"neobase-ai/internal/repositories"
	"neobase-ai/internal/services"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/mongodb"
	"neobase-ai/pkg/redis"
	"time"

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

	jwtService := utils.NewJWTService(config.Env.JWTSecret,
		time.Millisecond*time.Duration(config.Env.JWTExpirationMilliseconds),
		time.Millisecond*time.Duration(config.Env.JWTRefreshExpirationMilliseconds))
	// Provide dependencies
	DiContainer.Provide(redisRepo)
	DiContainer.Provide(mongodbClient)
	DiContainer.Provide(repositories.NewUserRepository)
	DiContainer.Provide(jwtService)
	DiContainer.Provide(services.NewAuthService)
	DiContainer.Provide(handlers.NewAuthHandler)
}
