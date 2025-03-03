package di

import (
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/apis/handlers"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/repositories"
	"neobase-ai/internal/services"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
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

	// Provide DB Manager
	if err := DiContainer.Provide(func(redisRepo redis.IRedisRepositories) (*dbmanager.Manager, error) {
		encryptionKey := config.Env.SchemaEncryptionKey
		manager, err := dbmanager.NewManager(redisRepo, encryptionKey)
		if err != nil {
			log.Fatalf("Failed to provide DB manager: %v", err)
		}
		// Register database drivers
		manager.RegisterDriver(constants.DatabaseTypePostgreSQL, dbmanager.NewPostgresDriver())
		manager.RegisterDriver(constants.DatabaseTypeYugabyteDB, dbmanager.NewPostgresDriver()) // Use same driver for both
		return manager, nil
	}); err != nil {
		log.Fatalf("Failed to provide DB manager: %v", err)
	}

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

	// Add LLM Manager
	if err := DiContainer.Provide(func() *llm.Manager {
		manager := llm.NewManager()

		switch config.Env.DefaultLLMClient {
		case constants.OpenAI:
			// Register default OpenAI client
			err := manager.RegisterClient(constants.OpenAI, llm.Config{
				Provider:            constants.OpenAI,
				Model:               config.Env.OpenAIModel,
				APIKey:              config.Env.OpenAIAPIKey,
				MaxCompletionTokens: config.Env.OpenAIMaxCompletionTokens,
				Temperature:         config.Env.OpenAITemperature,
				DBConfigs: []llm.LLMDBConfig{
					{
						DBType:       constants.DatabaseTypePostgreSQL,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypePostgreSQL),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypePostgreSQL),
					},
					{
						DBType:       constants.DatabaseTypeYugabyteDB,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeYugabyteDB),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeYugabyteDB),
					},
					{
						DBType:       constants.DatabaseTypeMySQL,
						Schema:       constants.GetLLMResponseSchema(constants.OpenAI, constants.DatabaseTypeMySQL),
						SystemPrompt: constants.GetSystemPrompt(constants.OpenAI, constants.DatabaseTypeMySQL),
					},
				},
			})
			if err != nil {
				log.Printf("Warning: Failed to register OpenAI client: %v", err)
			}
		case constants.Gemini:
			// Register default Gemini client
			err := manager.RegisterClient(constants.Gemini, llm.Config{
				Provider:            constants.Gemini,
				Model:               config.Env.GeminiModel,
				APIKey:              config.Env.GeminiAPIKey,
				MaxCompletionTokens: config.Env.GeminiMaxCompletionTokens,
				Temperature:         config.Env.GeminiTemperature,
				DBConfigs: []llm.LLMDBConfig{
					{
						DBType:       constants.DatabaseTypePostgreSQL,
						Schema:       constants.GetLLMResponseSchema(constants.Gemini, constants.DatabaseTypePostgreSQL),
						SystemPrompt: constants.GetSystemPrompt(constants.Gemini, constants.DatabaseTypePostgreSQL),
					},
					{
						DBType:       constants.DatabaseTypeYugabyteDB,
						Schema:       constants.GetLLMResponseSchema(constants.Gemini, constants.DatabaseTypeYugabyteDB),
						SystemPrompt: constants.GetSystemPrompt(constants.Gemini, constants.DatabaseTypeYugabyteDB),
					},
				},
			})
			if err != nil {
				log.Printf("Warning: Failed to register Gemini client: %v", err)
			}
		}
		return manager
	}); err != nil {
		log.Fatalf("Failed to provide LLM manager: %v", err)
	}

	// Update Chat Service provider to include DB manager setup
	if err := DiContainer.Provide(func(
		chatRepo repositories.ChatRepository,
		llmRepo repositories.LLMMessageRepository,
		dbManager *dbmanager.Manager,
		llmManager *llm.Manager,
	) services.ChatService {
		// Get default LLM client
		llmClient, err := llmManager.GetClient(config.Env.DefaultLLMClient)
		if err != nil {
			log.Printf("Warning: Failed to get default LLM client: %v", err)
		}

		chatService := services.NewChatService(chatRepo, llmRepo, dbManager, llmClient)

		// Set chat service as stream handler for DB manager
		dbManager.SetStreamHandler(chatService)

		return chatService
	}); err != nil {
		log.Fatalf("Failed to provide chat service: %v", err)
	}

	if err := DiContainer.Provide(func(redisRepo redis.IRedisRepositories) services.GitHubService {
		return services.NewGitHubService(redisRepo)
	}); err != nil {
		log.Fatalf("Failed to provide github handler: %v", err)
	}

	// Provide handlers
	if err := DiContainer.Provide(func(authService services.AuthService) *handlers.AuthHandler {
		return handlers.NewAuthHandler(authService)
	}); err != nil {
		log.Fatalf("Failed to provide auth handler: %v", err)
	}

	if err := DiContainer.Provide(func(chatService services.ChatService) *handlers.ChatHandler {
		return handlers.NewChatHandler(chatService)
	}); err != nil {
		log.Fatalf("Failed to provide chat handler: %v", err)
	}

	if err := DiContainer.Provide(func(githubService services.GitHubService) *handlers.GitHubHandler {
		return handlers.NewGitHubHandler(githubService)
	}); err != nil {
		log.Fatalf("Failed to provide github handler: %v", err)
	}

	// Add visualization service and handler
	if err := DiContainer.Provide(func(dbManager *dbmanager.Manager) services.VisualizationService {
		return services.NewVisualizationService(dbManager)
	}); err != nil {
		log.Fatalf("Failed to provide visualization service: %v", err)
	}

	if err := DiContainer.Provide(func(visualizationService services.VisualizationService) *handlers.VisualizationHandler {
		return handlers.NewVisualizationHandler(visualizationService)
	}); err != nil {
		log.Fatalf("Failed to provide visualization handler: %v", err)
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

// GetGitHubHandler retrieves the GitHubHandler from the DI container
func GetGitHubHandler() (*handlers.GitHubHandler, error) {
	var handler *handlers.GitHubHandler
	err := DiContainer.Invoke(func(h *handlers.GitHubHandler) {
		handler = h
	})
	return handler, err
}

// GetVisualizationHandler returns the visualization handler
func GetVisualizationHandler() (*handlers.VisualizationHandler, error) {
	var handler *handlers.VisualizationHandler
	err := DiContainer.Invoke(func(h *handlers.VisualizationHandler) {
		handler = h
	})
	return handler, err
}
