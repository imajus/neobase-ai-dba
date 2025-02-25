package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client              *openai.Client
	model               string
	maxCompletionTokens int
	temperature         float32
	DBConfigs           []LLMDBConfig
}

func NewOpenAIClient(config Config) (*OpenAIClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(config.APIKey)
	model := config.Model
	if model == "" {
		model = openai.GPT4o
	}

	return &OpenAIClient{
		client:              client,
		model:               model,
		maxCompletionTokens: config.MaxCompletionTokens,
		temperature:         config.Temperature,
		DBConfigs:           config.DBConfigs,
	}, nil
}

func (c *OpenAIClient) GenerateResponse(ctx context.Context, messages []*models.LLMMessage, dbType string) (string, error) {
	// Convert messages to OpenAI format
	openAIMessages := make([]openai.ChatCompletionMessage, 0, len(messages))

	systemPrompt := ""
	responseSchema := ""

	for _, dbConfig := range c.DBConfigs {
		if dbConfig.DBType == dbType {
			systemPrompt = dbConfig.SystemPrompt
			responseSchema = dbConfig.Schema
			break
		}
	}

	// Add system message with database-specific prompt only
	openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
		Role:    "system",
		Content: systemPrompt,
	})

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

		if content != "" {
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:    mapRole(msg.Role),
				Content: content,
			})
		}
	}

	// Create completion request with JSON schema
	req := openai.ChatCompletionRequest{
		Model:               c.model,
		Messages:            openAIMessages,
		MaxCompletionTokens: c.maxCompletionTokens,
		Temperature:         c.temperature,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        "neobase-response",
				Description: "A friendly AI Response/Explanation or clarification question (Must Send this)",
				Schema:      json.RawMessage(responseSchema),
				Strict:      false,
			},
		},
	}

	// Call OpenAI API
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("GenerateResponse -> err: %v", err)
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	log.Printf("GenerateResponse -> resp: %v", resp)
	// Validate response against schema
	var llmResponse constants.LLMResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &llmResponse); err != nil {
		return "", fmt.Errorf("invalid response format: %v", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) GetModelInfo() ModelInfo {
	return ModelInfo{
		Name:                c.model,
		Provider:            "openai",
		MaxCompletionTokens: c.maxCompletionTokens,
		ContextLimit:        getModelContextLimit(c.model),
	}
}

// Helper functions
func mapRole(role string) string {
	switch strings.ToLower(role) {
	case "user":
		return "user"
	case "assistant":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}

func getModelContextLimit(model string) int {
	switch model {
	case openai.GPT4TurboPreview:
		return 128000 // 128k tokens
	case openai.GPT4:
		return 8192 // 8k tokens
	case openai.GPT3Dot5Turbo:
		return 4096 // 4k tokens
	default:
		return 4096
	}
}
