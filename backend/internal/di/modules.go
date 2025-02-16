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

	// Initialize MongoDB
	dbConfig := mongodb.MongoDbConfigModel{
		ConnectionUrl: config.Env.MongoURI,
		DatabaseName:  config.Env.MongoDatabaseName,
	}
	mongodbClient := mongodb.InitializeDatabaseConnection(dbConfig)

	// Initialize Redis
	redisClient, err := redis.RedisClient(config.Env.RedisHost, config.Env.RedisPort, config.Env.RedisUsername, config.Env.RedisPassword)
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}

	// Initialize services and repositories
	redisRepo := redis.NewRedisRepositories(redisClient)
	jwtService := utils.NewJWTService(
		config.Env.JWTSecret,
		time.Millisecond*time.Duration(config.Env.JWTExpirationMilliseconds),
		time.Millisecond*time.Duration(config.Env.JWTRefreshExpirationMilliseconds),
	)

	// Initialize token repository
	tokenRepo := repositories.NewTokenRepository(redisRepo)

	chatRepo := repositories.NewChatRepository(mongodbClient)
	llmRepo := repositories.NewLLMMessageRepository(mongodbClient)

	// Provide all dependencies to the container
	if err := DiContainer.Provide(func() *mongodb.MongoDBClient { return mongodbClient }); err != nil {
		log.Fatalf("Failed to provide MongoDB client: %v", err)
	}

	if err := DiContainer.Provide(func() redis.IRedisRepositories { return redisRepo }); err != nil {
		log.Fatalf("Failed to provide Redis repositories: %v", err)
	}

	if err := DiContainer.Provide(func() utils.JWTService { return jwtService }); err != nil {
		log.Fatalf("Failed to provide JWT service: %v", err)
	}

	if err := DiContainer.Provide(func() repositories.ChatRepository { return chatRepo }); err != nil {
		log.Fatalf("Failed to provide chat repository: %v", err)
	}

	if err := DiContainer.Provide(func() repositories.LLMMessageRepository { return llmRepo }); err != nil {
		log.Fatalf("Failed to provide LLM message repository: %v", err)
	}

	// Provide repositories
	if err := DiContainer.Provide(func(db *mongodb.MongoDBClient) repositories.UserRepository {
		return repositories.NewUserRepository(db)
	}); err != nil {
		log.Fatalf("Failed to provide user repository: %v", err)
	}

	if err := DiContainer.Provide(func() repositories.TokenRepository { return tokenRepo }); err != nil {
		log.Fatalf("Failed to provide token repository: %v", err)
	}

	// Provide services
	if err := DiContainer.Provide(func(userRepo repositories.UserRepository, tokenRepo repositories.TokenRepository, jwt utils.JWTService) services.AuthService {
		return services.NewAuthService(userRepo, jwt, tokenRepo)
	}); err != nil {
		log.Fatalf("Failed to provide auth service: %v", err)
	}

	// Provide handlers
	if err := DiContainer.Provide(func(authService services.AuthService) *handlers.AuthHandler {
		return handlers.NewAuthHandler(authService)
	}); err != nil {
		log.Fatalf("Failed to provide auth handler: %v", err)
	}

	// Chat Repository
	if err := DiContainer.Provide(func(mongoClient *mongodb.MongoDBClient) repositories.ChatRepository {
		return repositories.NewChatRepository(mongoClient)
	}); err != nil {
		log.Fatalf("Failed to provide chat repository: %v", err)
	}

	// Chat Service
	if err := DiContainer.Provide(func(chatRepo repositories.ChatRepository, llmRepo repositories.LLMMessageRepository) services.ChatService {
		return services.NewChatService(chatRepo, llmRepo)
	}); err != nil {
		log.Fatalf("Failed to provide chat service: %v", err)
	}

	// Chat Handler
	if err := DiContainer.Provide(func(chatService services.ChatService) *handlers.ChatHandler {
		return handlers.NewChatHandler(chatService)
	}); err != nil {
		log.Fatalf("Failed to provide chat handler: %v", err)
	}
}

// GetAuthHandler retrieves the AuthHandler from the DI container
func GetAuthHandler() (*handlers.AuthHandler, error) {
	var handler *handlers.AuthHandler
	err := DiContainer.Invoke(func(h *handlers.AuthHandler) {
		handler = h
	})
	if err != nil {
		return nil, err
	}
	return handler, nil
}

// GetChatHandler retrieves the ChatHandler from the DI container
func GetChatHandler() (*handlers.ChatHandler, error) {
	var handler *handlers.ChatHandler
	err := DiContainer.Invoke(func(h *handlers.ChatHandler) {
		handler = h
	})
	if err != nil {
		return nil, err
	}
	return handler, nil
}
