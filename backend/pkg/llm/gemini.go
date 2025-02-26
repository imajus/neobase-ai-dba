package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/internal/utils"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiClient struct {
	client              *genai.Client
	model               string
	maxCompletionTokens int
	temperature         float64
	DBConfigs           []LLMDBConfig
}

func NewGeminiClient(config Config) (*GeminiClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("gemini API key is required")
	}
	// Create the Gemini SDK client using the provided API key.
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(config.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %v", err)
	}
	maxCompletionTokens := config.MaxCompletionTokens
	temperature := config.Temperature
	DBConfigs := config.DBConfigs

	return &GeminiClient{
		client:              client,
		model:               config.Model,
		maxCompletionTokens: maxCompletionTokens,
		temperature:         temperature,
		DBConfigs:           DBConfigs,
	}, nil
}

func (c *GeminiClient) GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error) {
	// Convert messages into parts for the Gemini API.
	geminiMessages := make([]*genai.Content, 0)

	systemPrompt := ""
	var responseSchema *genai.Schema

	for _, dbConfig := range c.DBConfigs {
		if dbConfig.DBType == dbType {
			systemPrompt = dbConfig.SystemPrompt
			responseSchema = dbConfig.Schema.(*genai.Schema)
			break
		}
	}

	log.Printf("GenerateResponse -> messages: %v", messages)

	for _, msg := range messages {
		content := ""

		// Handle different message types
		switch msg.Role {
		case "user":
			if userMsg, ok := msg.Content["user_message"].(string); ok {
				content = userMsg
			}
		case "assistant":
			content = formatAssistantResponse(msg.Content["assistant_response"].(map[string]interface{}))
		case "system":
			if schemaUpdate, ok := msg.Content["schema_update"].(string); ok {
				content = fmt.Sprintf("Database schema update:\n%s", schemaUpdate)
			}
		}

		userRole := "user"
		if msg.Role == "user" || msg.Role == "system" {
			userRole = "user"
		} else {
			userRole = "model"
		}
		if content != "" {
			geminiMessages = append(geminiMessages, &genai.Content{
				Role:  userRole,
				Parts: []genai.Part{genai.Text(content)},
			})
		}
	}
	// Build the request with a single content bundle.
	// Call Gemini's content generation API.
	model := c.client.GenerativeModel(c.model)
	model.MaxOutputTokens = utils.ToInt32Ptr(int32(c.maxCompletionTokens))
	model.SetTemperature(float32(c.temperature))
	model.ResponseMIMEType = "application/json"
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	model.ResponseSchema = responseSchema

	session := model.StartChat()
	session.History = geminiMessages
	result, err := session.SendMessage(ctx, genai.Text("answer the request"))
	if err != nil {
		log.Printf("gemini API error: %v", err)
		return "", fmt.Errorf("gemini API error: %v", err)
	}

	responseText := strings.ReplaceAll(fmt.Sprintf("%v", result.Candidates[0].Content.Parts[0]), "```json", "")
	responseText = strings.ReplaceAll(responseText, "```", "")
	var llmResponse constants.LLMResponse
	if errJSON := json.Unmarshal([]byte(responseText), &llmResponse); errJSON != nil {
		log.Printf("Warning: Gemini response didn't match expected JSON schema: %v", errJSON)
	}

	return responseText, nil
}

// GetModelInfo returns information about the Gemini model.
func (c *GeminiClient) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:                c.model,
		Provider:            "gemini",
		MaxCompletionTokens: c.maxCompletionTokens,
	}
}
