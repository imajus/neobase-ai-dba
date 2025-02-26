package constants

const (
	OpenAI = "openai"
	Gemini = "gemini"
)

func GetLLMResponseSchema(provider string, dbType string) interface{} {
	switch provider {
	case OpenAI:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return OpenAIPostgresLLMResponseSchema
		case DatabaseTypeYugabyteDB:
			return OpenAIYugabyteDBLLMResponseSchema
		default:
			return OpenAIPostgresLLMResponseSchema
		}
	case Gemini:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return GeminiPostgresLLMResponseSchema
		case DatabaseTypeYugabyteDB:
			return GeminiYugabyteDBLLMResponseSchema
		}
	}
	return ""
}

// GetSystemPrompt returns the appropriate system prompt based on database type
func GetSystemPrompt(provider string, dbType string) string {
	switch provider {
	case OpenAI:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return OpenAIPostgreSQLPrompt
		case DatabaseTypeMySQL:
			return OpenAIMySQLPrompt
		case DatabaseTypeYugabyteDB:
			return OpenAIYugabyteDBPrompt
		default:
			return OpenAIPostgreSQLPrompt // Default to PostgreSQL
		}
	case Gemini:
		switch dbType {
		case DatabaseTypePostgreSQL:
			return GeminiPostgreSQLPrompt
		case DatabaseTypeYugabyteDB:
			return GeminiYugabyteDBPrompt
		}
	}
	return ""
}
