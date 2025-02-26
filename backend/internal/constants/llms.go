package constants

const (
	OpenAI = "openai"
	Gemini = "gemini"
)

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
			return OpenAYugabyteDBPrompt
		default:
			return OpenAIPostgreSQLPrompt // Default to PostgreSQL
		}

	default:
		return ""
	}
}
