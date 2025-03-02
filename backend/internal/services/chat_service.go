package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/constants"
	"neobase-ai/internal/models"
	"neobase-ai/internal/repositories"
	"neobase-ai/internal/utils"
	"neobase-ai/pkg/dbmanager"
	"neobase-ai/pkg/llm"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Used by Handler
type StreamHandler interface {
	HandleStreamEvent(userID, chatID, streamID string, response dtos.StreamResponse)
}

type ChatService interface {
	Create(userID string, req *dtos.CreateChatRequest) (*dtos.ChatResponse, uint32, error)
	Update(userID, chatID string, req *dtos.UpdateChatRequest) (*dtos.ChatResponse, uint32, error)
	Delete(userID, chatID string) (uint32, error)
	GetByID(userID, chatID string) (*dtos.ChatResponse, uint32, error)
	List(userID string, page, pageSize int) (*dtos.ChatListResponse, uint32, error)
	CreateMessage(ctx context.Context, userID, chatID string, streamID string, content string) (*dtos.MessageResponse, uint16, error)
	UpdateMessage(ctx context.Context, userID, chatID, messageID string, streamID string, req *dtos.CreateMessageRequest) (*dtos.MessageResponse, uint32, error)
	DeleteMessages(userID, chatID string) (uint32, error)
	ListMessages(userID, chatID string, page, pageSize int) (*dtos.MessageListResponse, uint32, error)
	SetStreamHandler(handler StreamHandler)
	CancelProcessing(userID, chatID, streamID string)
	ConnectDB(ctx context.Context, userID, chatID string, streamID string) (uint32, error)
	DisconnectDB(ctx context.Context, userID, chatID string, streamID string) (uint32, error)
	GetDBConnectionStatus(ctx context.Context, userID, chatID string) (*dtos.ConnectionStatusResponse, uint32, error)
	ExecuteQuery(ctx context.Context, userID, chatID string, req *dtos.ExecuteQueryRequest) (*dtos.QueryExecutionResponse, uint32, error)
	RollbackQuery(ctx context.Context, userID, chatID string, req *dtos.RollbackQueryRequest) (*dtos.QueryExecutionResponse, uint32, error)
	CancelQueryExecution(userID, chatID, messageID, queryID, streamID string)
	ProcessMessage(ctx context.Context, userID, chatID string, streamID string) error
	RefreshSchema(ctx context.Context, userID, chatID string, sync bool) (uint32, error)
	// Db Manager Stream Handler
	HandleSchemaChange(userID, chatID, streamID string, diff *dbmanager.SchemaDiff)
	HandleDBEvent(userID, chatID, streamID string, response dtos.StreamResponse)
	GetQueryResults(ctx context.Context, userID, chatID, messageID, queryID, streamID string, offset int) (*dtos.QueryResultsResponse, uint32, error)
	EditQuery(ctx context.Context, userID, chatID, messageID, queryID string, query string) (*dtos.EditQueryResponse, uint32, error)
	// New method for getting tables
	GetTables(ctx context.Context, userID, chatID string) (*dtos.TablesResponse, uint32, error)
	// GetSelectedCollections retrieves the selected collections for a chat
	GetSelectedCollections(chatID string) (string, error)
}

type chatService struct {
	chatRepo        repositories.ChatRepository
	llmRepo         repositories.LLMMessageRepository
	dbManager       *dbmanager.Manager
	llmClient       llm.Client
	streamChans     map[string]chan dtos.StreamResponse
	streamHandler   StreamHandler
	activeProcesses map[string]context.CancelFunc // key: streamID
	processesMu     sync.RWMutex
}

func NewChatService(
	chatRepo repositories.ChatRepository,
	llmRepo repositories.LLMMessageRepository,
	dbManager *dbmanager.Manager,
	llmClient llm.Client,
) ChatService {
	return &chatService{
		chatRepo:        chatRepo,
		llmRepo:         llmRepo,
		dbManager:       dbManager,
		llmClient:       llmClient,
		streamChans:     make(map[string]chan dtos.StreamResponse),
		activeProcesses: make(map[string]context.CancelFunc),
	}
}

func (s *chatService) Create(userID string, req *dtos.CreateChatRequest) (*dtos.ChatResponse, uint32, error) {
	log.Printf("Creating chat for user %s", userID)

	// Check if the database type is supported
	if req.Connection.Type != constants.DatabaseTypePostgreSQL && req.Connection.Type != constants.DatabaseTypeYugabyteDB && req.Connection.Type != constants.DatabaseTypeMySQL && req.Connection.Type != constants.DatabaseTypeClickhouse {
		return nil, http.StatusBadRequest, fmt.Errorf("only PostgreSQL, YugabyteDB, MySQL and Clickhouse are supported for now")
	}

	// Test connection without creating a persistent connection
	err := s.dbManager.TestConnection(&dbmanager.ConnectionConfig{
		Type:     req.Connection.Type,
		Host:     req.Connection.Host,
		Port:     req.Connection.Port,
		Username: &req.Connection.Username,
		Password: &req.Connection.Password,
		Database: req.Connection.Database,
	})
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("connection failed: %v", err)
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}
	// Create connection object
	connection := models.Connection{
		Type:     req.Connection.Type,
		Host:     req.Connection.Host,
		Port:     req.Connection.Port,
		Username: &req.Connection.Username,
		Password: &req.Connection.Password,
		Database: req.Connection.Database,
		Base:     models.NewBase(),
	}

	// Create chat with connection
	chat := models.NewChat(userObjID, connection, req.AutoExecuteQuery)
	if err := s.chatRepo.Create(chat); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return s.buildChatResponse(chat), http.StatusCreated, nil
}

func (s *chatService) Update(userID, chatID string, req *dtos.UpdateChatRequest) (*dtos.ChatResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Get existing chat
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	if req.Connection != nil {
		// Cannot change the database type
		if req.Connection.Type != chat.Connection.Type {
			return nil, http.StatusBadRequest, fmt.Errorf("cannot change the database type")
		}

		// Test connection without creating a persistent connection
		err = s.dbManager.TestConnection(&dbmanager.ConnectionConfig{
			Type:     req.Connection.Type,
			Host:     req.Connection.Host,
			Port:     req.Connection.Port,
			Username: &req.Connection.Username,
			Password: &req.Connection.Password,
			Database: req.Connection.Database,
		})
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("connection failed: %v", err)
		}

		// Update chat fields
		if req.Connection != nil {
			chat.Connection = models.Connection{
				Type:     req.Connection.Type,
				Host:     req.Connection.Host,
				Port:     req.Connection.Port,
				Username: &req.Connection.Username,
				Password: &req.Connection.Password,
				Database: req.Connection.Database,
				Base:     models.NewBase(),
			}
		}
	}

	if req.AutoExecuteQuery != nil {
		log.Printf("ChatService -> Update -> AutoExecuteQuery: %v", *req.AutoExecuteQuery)
		chat.AutoExecuteQuery = *req.AutoExecuteQuery
	}

	// Store the old selected collections value to check for changes
	oldSelectedCollections := chat.SelectedCollections
	// Flag to track if selected collections changed
	selectedCollectionsChanged := false

	// Update selected collections if provided
	if req.SelectedCollections != nil && *req.SelectedCollections != "" {
		if oldSelectedCollections != *req.SelectedCollections {
			selectedCollectionsChanged = true
			log.Printf("ChatService -> Update -> Selected collections changed from '%s' to '%s'", oldSelectedCollections, *req.SelectedCollections)
		}
		chat.SelectedCollections = *req.SelectedCollections
		log.Printf("ChatService -> Update -> Updated selected collections to: %s", *req.SelectedCollections)
	}

	if err := s.chatRepo.Update(chatObjID, chat); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update chat: %v", err)
	}

	// If selected collections changed, trigger a schema refresh
	if selectedCollectionsChanged {
		log.Printf("ChatService -> Update -> Triggering schema refresh due to selected collections change")
		go func() {
			// Create a completely new context with a much longer timeout
			// This ensures it's not tied to the API request context
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
			defer cancel()

			log.Printf("ChatService -> Update -> Starting schema refresh with 60-minute timeout")
			_, err := s.RefreshSchema(ctx, userID, chatID, false)
			if err != nil {
				log.Printf("ChatService -> Update -> Error refreshing schema: %v", err)
			}
		}()
	}

	return s.buildChatResponse(chat), http.StatusOK, nil
}

func (s *chatService) Delete(userID, chatID string) (uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Verify ownership
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	// Delete chat and its messages
	if err := s.chatRepo.Delete(chatObjID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete chat: %v", err)
	}
	// We want to delete messages, except system messages
	if err := s.llmRepo.DeleteMessagesByChatID(chatObjID, false); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete chat messages: %v", err)
	}

	go func() {
		// Delete DB connection
		if err := s.dbManager.Disconnect(chatID, userID, true); err != nil {
			log.Printf("failed to delete DB connection: %v", err)
		}
	}()

	return http.StatusOK, nil
}

func (s *chatService) GetByID(userID, chatID string) (*dtos.ChatResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	return s.buildChatResponse(chat), http.StatusOK, nil
}

func (s *chatService) List(userID string, page, pageSize int) (*dtos.ChatListResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chats, total, err := s.chatRepo.FindByUserID(userObjID, page, pageSize)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chats: %v", err)
	}

	response := &dtos.ChatListResponse{
		Chats: make([]dtos.ChatResponse, len(chats)),
		Total: total,
	}

	for i, chat := range chats {
		response.Chats[i] = *s.buildChatResponse(chat)
	}

	return response, http.StatusOK, nil
}

// Add new types for message handling
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeSystem    MessageType = "system"
)

func (s *chatService) CreateMessage(ctx context.Context, userID, chatID string, streamID string, content string) (*dtos.MessageResponse, uint16, error) {
	// Validate chat exists and user has access
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}

	// Create and save the user message first
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	msg := &models.Message{
		Base:    models.NewBase(),
		UserID:  userObjID,
		ChatID:  chatObjID,
		Content: content,
		Type:    string(MessageTypeUser),
	}

	if err := s.chatRepo.CreateMessage(msg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to save message: %v", err)
	}

	// Make LLM Message
	llmMsg := &models.LLMMessage{
		Base:      models.NewBase(),
		UserID:    userObjID,
		ChatID:    chatObjID,
		MessageID: msg.ID,
		Role:      string(MessageTypeUser),
		Content: map[string]interface{}{
			"user_message": content,
		},
	}
	if err := s.llmRepo.CreateMessage(llmMsg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to save LLM message: %v", err)
	}

	log.Printf("ChatService -> CreateMessage -> AutoExecuteQuery: %v", chat.AutoExecuteQuery)
	// If auto execute query is true, we need to process LLM response & run query automatically
	if chat.AutoExecuteQuery {
		if err := s.ProcessLLMResponseAndRunQuery(ctx, userID, chatID, msg.ID.Hex(), streamID); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to process message: %v", err)
		}
	} else {
		// Start processing the message asynchronously
		if err := s.ProcessMessage(ctx, userID, chatID, streamID); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to process message: %v", err)
		}
	}

	// Return the actual message ID
	return &dtos.MessageResponse{
		ID:        msg.ID.Hex(), // Use actual message ID
		ChatID:    chatID,
		Content:   content,
		Type:      "user",
		CreatedAt: msg.CreatedAt.Format(time.RFC3339),
	}, http.StatusOK, nil
}

// processLLMResponse processes the LLM response updates SSE stream only if synchronous is false, allowSSEUpdates is used to send SSE updates to the client except the final ai-response event
func (s *chatService) processLLMResponse(ctx context.Context, userID, chatID, streamID string, synchronous bool, allowSSEUpdates bool) (*dtos.MessageResponse, error) {
	log.Printf("processLLMResponse -> userID: %s, chatID: %s, streamID: %s", userID, chatID, streamID)

	// Create cancellable context from the background context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, err
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, err
	}

	// Store cancel function
	s.processesMu.Lock()
	s.activeProcesses[streamID] = cancel
	s.processesMu.Unlock()

	// Cleanup when done
	defer func() {
		s.processesMu.Lock()
		delete(s.activeProcesses, streamID)
		s.processesMu.Unlock()
	}()

	if !synchronous || allowSSEUpdates {
		// Send initial processing message
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "NeoBase is analyzing your request..",
		})
	}

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		s.handleError(ctx, chatID, fmt.Errorf("connection info not found"))
		return nil, fmt.Errorf("connection info not found")
	}

	// Fetch all the messages from the LLM
	messages, err := s.llmRepo.GetByChatID(chatObjID)
	if err != nil {
		s.handleError(ctx, chatID, err)
		return nil, err
	}

	// Helper function to check cancellation
	checkCancellation := func() bool {
		select {
		case <-ctx.Done():
			if !synchronous || allowSSEUpdates {
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "response-cancelled",
					Data:  "Operation cancelled by user",
				})
			}
			return true
		default:
			return false
		}
	}

	// Check cancellation before expensive operations
	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	if !synchronous || allowSSEUpdates {
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "Fetching relevant data points & structure for the query..",
		})

		// Send initial processing message
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "Generating an optimized query & example results for the request..",
		})
	}
	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	// Generate LLM response
	response, err := s.llmClient.GenerateResponse(ctx, messages, connInfo.Config.Type)
	if err != nil {
		if !synchronous || allowSSEUpdates {
			s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
				Event: "ai-response-error",
				Data:  map[string]string{"error": err.Error()},
			})
		}
		return nil, fmt.Errorf("failed to generate LLM response: %v", err)
	}

	log.Printf("processLLMResponse -> response: %s", response)

	if checkCancellation() {
		return nil, fmt.Errorf("operation cancelled")
	}

	// Send initial processing message
	if !synchronous || allowSSEUpdates {
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-step",
			Data:  "Analyzing the criticality of the query & if roll back is possible..",
		})
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response-error",
			Data:  map[string]string{"error": err.Error()},
		})
	}

	queries := []models.Query{}
	if jsonResponse["queries"] != nil {
		for _, query := range jsonResponse["queries"].([]interface{}) {
			queryMap := query.(map[string]interface{})
			var exampleResult *string
			log.Printf("processLLMResponse -> queryMap: %v", queryMap)
			if queryMap["exampleResult"] != nil {
				log.Printf("processLLMResponse -> queryMap[\"exampleResult\"]: %v", queryMap["exampleResult"])
				result, _ := json.Marshal(queryMap["exampleResult"].([]interface{}))
				exampleResult = utils.ToStringPtr(string(result))
				log.Printf("processLLMResponse -> saving exampleResult: %v", *exampleResult)
			} else {
				exampleResult = nil
				log.Println("processLLMResponse -> saving exampleResult: nil")
			}

			var rollbackDependentQuery *string
			if queryMap["rollbackDependentQuery"] != nil {
				rollbackDependentQuery = utils.ToStringPtr(queryMap["rollbackDependentQuery"].(string))
			} else {
				rollbackDependentQuery = nil
			}

			var estimateResponseTime *float64
			// First check if the estimateResponseTime is a string, if not string & it is float then set value
			if queryMap["estimateResponseTime"] != nil {
				switch v := queryMap["estimateResponseTime"].(type) {
				case string:
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						estimateResponseTime = &f
					} else {
						defaultVal := float64(100)
						estimateResponseTime = &defaultVal
					}
				case float64:
					estimateResponseTime = &v
				default:
					defaultVal := float64(100)
					estimateResponseTime = &defaultVal
				}
			} else {
				defaultVal := float64(100)
				estimateResponseTime = &defaultVal
			}

			log.Printf("processLLMResponse -> queryMap[\"pagination\"]: %v", queryMap["pagination"])
			pagination := &models.Pagination{}
			if queryMap["pagination"] != nil {
				pagination.PaginatedQuery = utils.ToStringPtr(queryMap["pagination"].(map[string]interface{})["paginatedQuery"].(string))
				log.Printf("processLLMResponse -> pagination.PaginatedQuery: %v", *pagination.PaginatedQuery)
			}
			var tables *string
			if queryMap["tables"] != nil {
				tables = utils.ToStringPtr(queryMap["tables"].(string))
			}
			var queryType *string
			if queryMap["queryType"] != nil {
				queryType = utils.ToStringPtr(queryMap["queryType"].(string))
			}

			var rollbackQuery *string
			if queryMap["rollbackQuery"] != nil {
				rollbackQuery = utils.ToStringPtr(queryMap["rollbackQuery"].(string))
			}

			// Create the query object
			query := models.Query{
				ID:                     primitive.NewObjectID(),
				Query:                  queryMap["query"].(string),
				Description:            queryMap["explanation"].(string),
				ExecutionTime:          nil,
				ExampleExecutionTime:   int(*estimateResponseTime),
				CanRollback:            queryMap["canRollback"].(bool),
				IsCritical:             queryMap["isCritical"].(bool),
				IsExecuted:             false,
				IsRolledBack:           false,
				ExampleResult:          exampleResult,
				ExecutionResult:        nil,
				Error:                  nil,
				QueryType:              queryType,
				Tables:                 tables,
				RollbackQuery:          rollbackQuery,
				RollbackDependentQuery: rollbackDependentQuery,
				Pagination:             pagination,
			}

			// Handle ClickHouse-specific metadata
			if connInfo.Config.Type == constants.DatabaseTypeClickhouse {
				metadata := make(map[string]interface{})

				// Add ClickHouse-specific fields if they exist
				if queryMap["engineType"] != nil {
					metadata["engineType"] = queryMap["engineType"]
				}
				if queryMap["partitionKey"] != nil {
					metadata["partitionKey"] = queryMap["partitionKey"]
				}
				if queryMap["orderByKey"] != nil {
					metadata["orderByKey"] = queryMap["orderByKey"]
				}

				// Store metadata as JSON if we have any
				if len(metadata) > 0 {
					metadataJSON, err := json.Marshal(metadata)
					if err == nil {
						metadataStr := string(metadataJSON)
						query.Metadata = &metadataStr
					}
				}
			}

			queries = append(queries, query)
		}
	}

	log.Printf("processLLMResponse -> queries: %v", queries)

	assistantMessage := ""
	if jsonResponse["assistantMessage"] != nil {
		assistantMessage = jsonResponse["assistantMessage"].(string)
	} else {
		assistantMessage = ""
	}

	// Save response and send final message
	chatResponseMsg := models.NewMessage(userObjID, chatObjID, string(MessageTypeAssistant), assistantMessage, &queries)
	if err := s.chatRepo.CreateMessage(chatResponseMsg); err != nil {
		log.Printf("processLLMResponse -> Error saving chat response message: %v", err)
		return nil, err
	}

	formattedJsonResponse := map[string]interface{}{
		"assistant_response": jsonResponse,
	}
	llmMsg := &models.LLMMessage{
		Base:      models.NewBase(),
		UserID:    userObjID,
		ChatID:    chatObjID,
		MessageID: chatResponseMsg.ID,
		Content:   formattedJsonResponse,
		Role:      string(MessageTypeAssistant),
	}
	if err := s.llmRepo.CreateMessage(llmMsg); err != nil {
		log.Printf("processLLMResponse -> Error saving LLM message: %v", err)
	}

	if !synchronous {
		// Send final response
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "ai-response",
			Data: &dtos.MessageResponse{
				ID:        chatResponseMsg.ID.Hex(),
				ChatID:    chatResponseMsg.ChatID.Hex(),
				Content:   chatResponseMsg.Content,
				Queries:   dtos.ToQueryDto(chatResponseMsg.Queries),
				Type:      chatResponseMsg.Type,
				CreatedAt: chatResponseMsg.CreatedAt.Format(time.RFC3339),
			},
		})
	}
	return &dtos.MessageResponse{
		ID:        chatResponseMsg.ID.Hex(),
		ChatID:    chatResponseMsg.ChatID.Hex(),
		Content:   chatResponseMsg.Content,
		Queries:   dtos.ToQueryDto(chatResponseMsg.Queries),
		Type:      chatResponseMsg.Type,
		CreatedAt: chatResponseMsg.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *chatService) handleError(_ context.Context, chatID string, err error) {
	log.Printf("Error processing message for chat %s: %v", chatID, err)
}

func (s *chatService) UpdateMessage(ctx context.Context, userID, chatID, messageID string, streamID string, req *dtos.CreateMessageRequest) (*dtos.MessageResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	messageObjID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid message ID format")
	}

	message, err := s.chatRepo.FindMessageByID(messageObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch message: %v", err)
	}

	if message.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized access to message")
	}

	if message.ChatID != chatObjID {
		return nil, http.StatusBadRequest, fmt.Errorf("message does not belong to chat")
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}

	log.Printf("UpdateMessage -> content: %+v", req.Content)
	// Update message content, This is a user message
	message.Content = req.Content
	message.IsEdited = true
	log.Printf("UpdateMessage -> message: %+v", message)
	log.Printf("UpdateMessage -> message.Content: %+v", message.Content)
	err = s.chatRepo.UpdateMessage(message.ID, message)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update message: %v", err)
	}

	llmMsg, err := s.llmRepo.FindMessageByChatMessageID(message.ID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch LLM message: %v", err)
	}

	log.Printf("UpdateMessage -> llmMsg: %+v", llmMsg)
	llmMsg.Content = map[string]interface{}{
		"user_message": req.Content,
	}

	if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update LLM message: %v", err)
	}

	// If auto execute query is true, we need to process LLM response & run query automatically
	if chat.AutoExecuteQuery {
		if err := s.ProcessLLMResponseAndRunQuery(ctx, userID, chatID, messageID, streamID); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to process message: %v", err)
		}
	} else {
		// Start processing the message asynchronously
		if err := s.ProcessMessage(ctx, userID, chatID, streamID); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to process message: %v", err)
		}
	}
	return s.buildMessageResponse(message), http.StatusOK, nil
}

func (s *chatService) DeleteMessages(userID, chatID string) (uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Verify chat ownership
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	if err := s.chatRepo.DeleteMessages(chatObjID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete messages: %v", err)
	}

	// Delete LLM messages
	if err := s.llmRepo.DeleteMessagesByChatID(chatObjID, true); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete LLM messages: %v", err)
	}

	return http.StatusOK, nil
}

func (s *chatService) ListMessages(userID, chatID string, page, pageSize int) (*dtos.MessageListResponse, uint32, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	// Verify chat ownership
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, http.StatusNotFound, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return nil, http.StatusForbidden, fmt.Errorf("unauthorized access to chat")
	}

	messages, total, err := s.chatRepo.FindLatestMessageByChat(chatObjID, page, pageSize)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch messages: %v", err)
	}

	response := &dtos.MessageListResponse{
		Messages: make([]dtos.MessageResponse, len(messages)),
		Total:    total,
	}

	for i, msg := range messages {
		response.Messages[i] = *s.buildMessageResponse(msg)
	}

	return response, http.StatusOK, nil
}

// Helper methods for building responses
func (s *chatService) buildChatResponse(chat *models.Chat) *dtos.ChatResponse {
	return &dtos.ChatResponse{
		ID:     chat.ID.Hex(),
		UserID: chat.UserID.Hex(),
		Connection: dtos.ConnectionResponse{
			ID:       chat.ID.Hex(),
			Type:     chat.Connection.Type,
			Host:     chat.Connection.Host,
			Port:     chat.Connection.Port,
			Username: *chat.Connection.Username,
			Database: chat.Connection.Database,
		},
		SelectedCollections: chat.SelectedCollections,
		AutoExecuteQuery:    chat.AutoExecuteQuery,
		CreatedAt:           chat.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           chat.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *chatService) buildMessageResponse(msg *models.Message) *dtos.MessageResponse {
	return &dtos.MessageResponse{
		ID:        msg.ID.Hex(),
		ChatID:    msg.ChatID.Hex(),
		Type:      msg.Type,
		Content:   msg.Content,
		IsEdited:  msg.IsEdited,
		Queries:   dtos.ToQueryDto(msg.Queries),
		CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *chatService) SetStreamHandler(handler StreamHandler) {
	s.streamHandler = handler
}

// Helper method to send stream events
func (s *chatService) sendStreamEvent(userID, chatID, streamID string, response dtos.StreamResponse) {
	log.Printf("sendStreamEvent -> userID: %s, chatID: %s, streamID: %s, response: %+v", userID, chatID, streamID, response)
	if s.streamHandler != nil {
		s.streamHandler.HandleStreamEvent(userID, chatID, streamID, response)
	} else {
		log.Printf("sendStreamEvent -> no stream handler set")
	}
}

// Add method to cancel processing
func (s *chatService) CancelProcessing(userID, chatID, streamID string) {
	s.processesMu.Lock()
	defer s.processesMu.Unlock()

	log.Printf("CancelProcessing -> activeProcesses: %+v", s.activeProcesses)
	if cancel, exists := s.activeProcesses[streamID]; exists {
		log.Printf("CancelProcessing -> canceling LLM processing for streamID: %s", streamID)
		cancel() // Only cancels the LLM context
		delete(s.activeProcesses, streamID)

		go func() {
			chatObjID, err := primitive.ObjectIDFromHex(chatID)
			if err != nil {
				log.Printf("CancelProcessing -> error fetching chatID: %v", err)
			}

			userObjID, err := primitive.ObjectIDFromHex(userID)
			if err != nil {
				log.Printf("CancelProcessing -> error fetching userID: %v", err)
			}

			msg := &models.Message{
				Base:    models.NewBase(),
				ChatID:  chatObjID,
				UserID:  userObjID,
				Type:    string(MessageTypeAssistant),
				Content: "Operation cancelled by user",
			}

			// Save cancelled event to database
			if err := s.chatRepo.CreateMessage(msg); err != nil {
				log.Printf("CancelProcessing -> error creating message: %v", err)
			}
		}()
		// Send cancelled event using stream
		s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
			Event: "response-cancelled",
			Data:  "Operation cancelled by user",
		})
	}
}

func (s *chatService) ConnectDB(ctx context.Context, userID, chatID string, streamID string) (uint32, error) {
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
	}

	chatDetails, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
	}

	log.Printf("ChatService -> ConnectDB -> chatDetails: %+v", chatDetails)

	// Validate connection type
	if !isValidDBType(chatDetails.Connection.Type) {
		return http.StatusBadRequest, fmt.Errorf("unsupported database type")
	}

	// Subscribe to connection status updates before connecting
	s.dbManager.Subscribe(chatID, streamID)

	// Attempt to connect
	if err := s.dbManager.Connect(chatID, userID, streamID, dbmanager.ConnectionConfig{
		Type:     chatDetails.Connection.Type,
		Host:     chatDetails.Connection.Host,
		Port:     chatDetails.Connection.Port,
		Username: chatDetails.Connection.Username,
		Password: chatDetails.Connection.Password,
		Database: chatDetails.Connection.Database,
	}); err != nil {
		log.Printf("ChatService -> ConnectDB -> error: %v", err)
		if strings.Contains(err.Error(), "already exists") {
			return http.StatusOK, nil
		}
		return http.StatusBadRequest, err
	}

	log.Printf("ChatService -> ConnectDB -> connected to chat: %s", chatID)

	// Send to stream handler
	s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
		Event: "db-connected",
		Data:  "Database connected successfully",
	})

	return http.StatusOK, nil
}

// Add method to handle DB status events
func (s *chatService) HandleDBEvent(userID, chatID, streamID string, response dtos.StreamResponse) {
	// Send to stream handler
	log.Printf("ChatService -> HandleDBEvent -> response: %+v", response)
	if s.streamHandler != nil {
		s.streamHandler.HandleStreamEvent(userID, chatID, streamID, response)
	}
}

// HandleSchemaChange handles schema changes
func (s *chatService) HandleSchemaChange(userID, chatID, streamID string, diff *dbmanager.SchemaDiff) {
	log.Printf("ChatService -> HandleSchemaChange -> Starting for chatID: %s", chatID)

	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		log.Printf("ChatService -> HandleSchemaChange -> Connection not found for chat ID: %s", chatID)
		return
	}

	// Get database connection
	dbConn, err := s.dbManager.GetConnection(chatID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Failed to get database connection: %v", err)
		return
	}

	// Get chat to get selected collections
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Error getting chatID: %v", err)
		return
	}

	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Error finding chat: %v", err)
		return
	}

	if chat == nil {
		log.Printf("ChatService -> HandleSchemaChange -> Chat not found for chatID: %s", chatID)
		return
	}

	// Convert the selectedCollections string to a slice
	var selectedCollectionsSlice []string
	if chat.SelectedCollections != "ALL" && chat.SelectedCollections != "" {
		selectedCollectionsSlice = strings.Split(chat.SelectedCollections, ",")
	}
	log.Printf("ChatService -> HandleSchemaChange -> Selected collections: %v", selectedCollectionsSlice)

	// Convert to ObjectID
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Invalid user ID format: %v", err)
		return
	}

	// Convert chat ID to ObjectID
	chatObjID, err = primitive.ObjectIDFromHex(chatID)
	if err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Invalid chat ID format: %v", err)
		return
	}

	// Clear previous system message from LLM
	if err := s.llmRepo.DeleteMessagesByRole(chatObjID, string(MessageTypeSystem)); err != nil {
		log.Printf("ChatService -> HandleSchemaChange -> Error deleting system message: %v", err)
	}

	// Format the schema changes for LLM
	if diff != nil {
		log.Printf("ChatService -> HandleSchemaChange -> diff: %+v", diff)

		// Need to update the chat LLM messages with the new schema
		// Only do full schema comparison if changes detected
		ctx := context.Background()
		var schemaMsg string
		if diff.IsFirstTime {
			// For first time, format the full schema with examples
			schemaMsg, err = s.dbManager.FormatSchemaWithExamples(ctx, chatID, selectedCollectionsSlice)
			if err != nil {
				log.Printf("ChatService -> HandleSchemaChange -> Error formatting schema with examples: %v", err)
				// Fall back to the old method if there's an error
				schemaMsg = s.dbManager.GetSchemaManager().FormatSchemaForLLM(diff.FullSchema)
			}
		} else {
			// For subsequent changes, get current schema with examples and show changes
			schemaMsg, err = s.dbManager.FormatSchemaWithExamples(ctx, chatID, selectedCollectionsSlice)
			if err != nil {
				log.Printf("ChatService -> HandleSchemaChange -> Error formatting schema with examples: %v", err)
				// Fall back to the old method if there's an error, but still use selected collections
				schema, schemaErr := s.dbManager.GetSchemaManager().GetSchema(ctx, chatID, dbConn, connInfo.Config.Type, selectedCollectionsSlice)
				if schemaErr != nil {
					log.Printf("ChatService -> HandleSchemaChange -> Error getting schema: %v", schemaErr)
					return
				}
				schemaMsg = s.dbManager.GetSchemaManager().FormatSchemaForLLM(schema)
			}
		}

		// Create LLM message with schema
		llmMsg := &models.LLMMessage{
			Base:   models.NewBase(),
			UserID: userObjID,
			ChatID: chatObjID,
			Role:   string(MessageTypeSystem),
			Content: map[string]interface{}{
				"schema_update": schemaMsg,
			},
		}

		// Save LLM message
		if err := s.llmRepo.CreateMessage(llmMsg); err != nil {
			log.Printf("ChatService -> HandleSchemaChange -> Error saving LLM message: %v", err)
			return
		}

		log.Printf("ChatService -> HandleSchemaChange -> Schema update message saved")
	}
}

func (s *chatService) DisconnectDB(ctx context.Context, userID, chatID string, streamID string) (uint32, error) {
	log.Printf("ChatService -> DisconnectDB -> Starting for chatID: %s", chatID)

	// Subscribe to connection status updates before disconnecting
	s.dbManager.Subscribe(chatID, streamID)
	log.Printf("ChatService -> DisconnectDB -> Subscribed to updates with streamID: %s", streamID)

	if err := s.dbManager.Disconnect(chatID, userID, false); err != nil {
		log.Printf("ChatService -> DisconnectDB -> failed to disconnect: %v", err)
		return http.StatusBadRequest, fmt.Errorf("failed to disconnect: %v", err)
	}

	log.Printf("ChatService -> DisconnectDB -> disconnected from chat: %s", chatID)
	return http.StatusOK, nil
}

func (s *chatService) GetDBConnectionStatus(ctx context.Context, userID, chatID string) (*dtos.ConnectionStatusResponse, uint32, error) {
	// Get connection info
	connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
	if !exists {
		return nil, http.StatusNotFound, fmt.Errorf("no connection found")
	}

	// Check if connection is active
	isConnected := s.dbManager.IsConnected(chatID)

	// Convert port string to int
	port, err := strconv.Atoi(connInfo.Config.Port)
	if err != nil {
		port = 0 // Default value if conversion fails
	}

	return &dtos.ConnectionStatusResponse{
		IsConnected: isConnected,
		Type:        connInfo.Config.Type,
		Host:        connInfo.Config.Host,
		Port:        port,
		Database:    connInfo.Config.Database,
		Username:    *connInfo.Config.Username,
	}, http.StatusOK, nil
}

func isValidDBType(dbType string) bool {
	validTypes := []string{constants.DatabaseTypePostgreSQL, constants.DatabaseTypeYugabyteDB, constants.DatabaseTypeYugabyteDB, constants.DatabaseTypeMySQL, constants.DatabaseTypeMongoDB, constants.DatabaseTypeClickhouse} // Add more as supported
	for _, t := range validTypes {
		if t == dbType {
			return true
		}
	}
	return false
}

func (s *chatService) ExecuteQuery(ctx context.Context, userID, chatID string, req *dtos.ExecuteQueryRequest) (*dtos.QueryExecutionResponse, uint32, error) {
	// Verify message and query ownership
	msg, query, err := s.verifyQueryOwnership(userID, chatID, req.MessageID, req.QueryID)
	if err != nil {
		return nil, http.StatusForbidden, err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, http.StatusRequestTimeout, fmt.Errorf("query execution cancelled or timed out")
	default:
		log.Printf("ChatService -> ExecuteQuery -> msg: %+v", msg)
		log.Printf("ChatService -> ExecuteQuery -> query: %+v", query)
	}

	// Check connection status and connect if needed
	if !s.dbManager.IsConnected(chatID) {
		log.Printf("ChatService -> ExecuteQuery -> Database not connected, initiating connection")
		status, err := s.ConnectDB(ctx, userID, chatID, req.StreamID)
		if err != nil {
			return nil, status, fmt.Errorf("failed to connect to database: %v", err)
		}
		// Give a small delay for connection to stabilize
		time.Sleep(1 * time.Second)
	}

	// Execute query
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, query.Query, *query.QueryType, false)
	if queryErr != nil {
		log.Printf("ChatService -> ExecuteQuery -> queryErr: %+v", queryErr)
		if queryErr.Code == "FAILED_TO_START_TRANSACTION" || strings.Contains(queryErr.Message, "context deadline exceeded") || strings.Contains(queryErr.Message, "context canceled") {
			return nil, http.StatusRequestTimeout, fmt.Errorf("query execution timed out")
		}
		go func() {
			log.Printf("ChatService -> ExecuteQuery -> Updating message")

			// Update query status in message
			if msg.Queries != nil {
				for i := range *msg.Queries {
					// Convert ObjectID to hex string for comparison
					queryIDHex := query.ID.Hex()
					msgQueryIDHex := (*msg.Queries)[i].ID.Hex()

					if msgQueryIDHex == queryIDHex {
						(*msg.Queries)[i].IsRolledBack = false
						(*msg.Queries)[i].IsExecuted = true
						(*msg.Queries)[i].ExecutionTime = nil
						(*msg.Queries)[i].Error = &models.QueryError{
							Code:    queryErr.Code,
							Message: queryErr.Message,
							Details: queryErr.Details,
						}
						break
					}
				}
			}

			// Save updated message
			if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error updating message: %v", err)
			}

			// Update LLM message with query execution results
			llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error finding LLM message: %v", err)
			} else if llmMsg != nil {
				log.Printf("ChatService -> ExecuteQuery -> llmMsg: %+v", llmMsg)

				content := llmMsg.Content
				if content == nil {
					content = make(map[string]interface{})
				}

				if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
					log.Printf("ChatService -> ExecuteQuery -> assistantResponse: %+v", assistantResponse)
					log.Printf("ChatService -> ExecuteQuery -> queries type: %T", assistantResponse["queries"])

					// Handle primitive.A (BSON array) type
					switch queriesVal := assistantResponse["queries"].(type) {
					case primitive.A:
						log.Printf("ChatService -> ExecuteQuery -> queries is primitive.A")
						// Convert primitive.A to []interface{}
						queries := make([]interface{}, len(queriesVal))
						for i, q := range queriesVal {
							if queryMap, ok := q.(map[string]interface{}); ok {
								// Compare hex strings of ObjectIDs
								if queryMap["id"] == query.ID.Hex() {
									queryMap["isRolledBack"] = false
									queryMap["executionTime"] = nil
									queryMap["error"] = map[string]interface{}{
										"code":    queryErr.Code,
										"message": queryErr.Message,
										"details": queryErr.Details,
									}
								}
								queries[i] = queryMap
							} else {
								queries[i] = q
							}
						}
						assistantResponse["queries"] = queries

					case []interface{}:
						log.Printf("ChatService -> ExecuteQuery -> queries is []interface{}")
						for i, q := range queriesVal {
							if queryMap, ok := q.(map[string]interface{}); ok {
								if queryMap["id"] == query.ID.Hex() {
									queryMap["isRolledBack"] = false
									queryMap["executionTime"] = query.ExecutionTime
									queryMap["error"] = map[string]interface{}{
										"code":    queryErr.Code,
										"message": queryErr.Message,
										"details": queryErr.Details,
									}
									queriesVal[i] = queryMap
								}
							}
						}
						assistantResponse["queries"] = queriesVal
					}

					content["assistant_response"] = assistantResponse
				}

				llmMsg.Content = content
				if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
					log.Printf("ChatService -> ExecuteQuery -> Error updating LLM message: %v", err)
				}
			}
		}()

		return &dtos.QueryExecutionResponse{
			ChatID:            chatID,
			MessageID:         msg.ID.Hex(),
			QueryID:           query.ID.Hex(),
			IsExecuted:        false,
			IsRolledBack:      false,
			ExecutionTime:     query.ExecutionTime,
			ExecutionResult:   nil,
			Error:             queryErr,
			TotalRecordsCount: nil,
		}, http.StatusOK, nil
	}
	var totalRecordsCount *int
	// Checking if the result record is a list with > 50 records, then cap it to 50 records.
	// Then we need to save capped 50 results in DB and the original records count to pagination.totalRecordsCount...
	log.Printf("ChatService -> ExecuteQuery -> result: %+v", result)
	log.Printf("ChatService -> ExecuteQuery -> result.ResultJSON: %+v", result.ResultJSON)

	var formattedResultJSON interface{}
	var resultListFormatting []interface{} = []interface{}{}
	var resultMapFormatting map[string]interface{} = map[string]interface{}{}
	if err := json.Unmarshal([]byte(result.ResultJSON), &resultListFormatting); err != nil {
		log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
		if err := json.Unmarshal([]byte(result.ResultJSON), &resultMapFormatting); err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
			// Try to unmarshal as a map
			err = json.Unmarshal([]byte(result.ResultJSON), &resultMapFormatting)
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error unmarshalling result JSON: %v", err)
			}
		}
	}

	log.Printf("ChatService -> ExecuteQuery -> resultListFormatting: %+v", resultListFormatting)
	log.Printf("ChatService -> ExecuteQuery -> resultMapFormatting: %+v", resultMapFormatting)
	if len(resultListFormatting) > 0 {
		log.Printf("ChatService -> ExecuteQuery -> resultListFormatting: %+v", resultListFormatting)
		formattedResultJSON = resultListFormatting
		if len(resultListFormatting) > 50 {
			log.Printf("ChatService -> ExecuteQuery -> resultListFormatting length > 50")
			totalRecordsCount = utils.ToIntPtr(len(resultListFormatting)) // Store actual total count
			formattedResultJSON = resultListFormatting[:50]               // Cap the result to 50 records

			// Cap the result.ResultJSON to 50 records
			cappedResults, err := json.Marshal(resultListFormatting[:50])
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error marshaling capped results: %v", err)
			} else {
				result.ResultJSON = string(cappedResults)
			}
		} else {
			totalRecordsCount = utils.ToIntPtr(len(resultListFormatting))
		}
	} else if resultMapFormatting != nil && resultMapFormatting["results"] != nil && len(resultMapFormatting["results"].([]interface{})) > 0 {
		log.Printf("ChatService -> ExecuteQuery -> resultMapFormatting: %+v", resultMapFormatting)
		totalRecordsCount = utils.ToIntPtr(len(resultMapFormatting["results"].([]interface{})))
		if len(resultMapFormatting["results"].([]interface{})) > 50 {
			formattedResultJSON = map[string]interface{}{
				"results": resultMapFormatting["results"].([]interface{})[:50],
			}
			cappedResults := map[string]interface{}{
				"results": resultMapFormatting["results"].([]interface{})[:50],
			}
			cappedResultsJSON, err := json.Marshal(cappedResults)
			if err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error marshaling capped results: %v", err)
			} else {
				result.ResultJSON = string(cappedResultsJSON)
			}
		} else {
			formattedResultJSON = map[string]interface{}{
				"results": resultMapFormatting["results"].([]interface{}),
			}
		}
	} else {
		formattedResultJSON = resultMapFormatting
	}

	log.Printf("ChatService -> ExecuteQuery -> totalRecordsCount: %+v", totalRecordsCount)
	log.Printf("ChatService -> ExecuteQuery -> formattedResultJSON: %+v", formattedResultJSON)

	query.IsExecuted = true
	query.IsRolledBack = false
	query.ExecutionTime = &result.ExecutionTime
	query.ExecutionResult = &result.ResultJSON
	if totalRecordsCount != nil {
		if query.Pagination == nil {
			query.Pagination = &models.Pagination{}
		}
		query.Pagination.TotalRecordsCount = totalRecordsCount
	}
	if result.Error != nil {
		query.Error = &models.QueryError{
			Code:    result.Error.Code,
			Message: result.Error.Message,
			Details: result.Error.Details,
		}
	} else {
		query.Error = nil
	}

	go func() {
		// Update query status in message
		if msg.Queries != nil {
			for i := range *msg.Queries {
				if (*msg.Queries)[i].ID == query.ID {
					(*msg.Queries)[i].IsRolledBack = false
					(*msg.Queries)[i].IsExecuted = true
					(*msg.Queries)[i].ExecutionTime = &result.ExecutionTime
					if totalRecordsCount != nil {
						if (*msg.Queries)[i].Pagination == nil {
							(*msg.Queries)[i].Pagination = &models.Pagination{}
						}
						(*msg.Queries)[i].Pagination.TotalRecordsCount = totalRecordsCount
					}
					(*msg.Queries)[i].ExecutionResult = &result.ResultJSON
					if result.Error != nil {
						(*msg.Queries)[i].Error = &models.QueryError{
							Code:    result.Error.Code,
							Message: result.Error.Message,
							Details: result.Error.Details,
						}
					} else {
						(*msg.Queries)[i].Error = nil
					}
					break
				}
			}
		}

		log.Printf("ChatService -> ExecuteQuery -> Updating message")
		// Save updated message
		if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error updating message: %v", err)
		}

		// Update LLM message with query execution results
		llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
		if err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error finding LLM message: %v", err)
		} else if llmMsg != nil {
			// Get the existing content
			content := llmMsg.Content
			if content == nil {
				content = make(map[string]interface{})
			}

			if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
				log.Printf("ChatService -> ExecuteQuery -> assistantResponse: %+v", assistantResponse)
				log.Printf("ChatService -> ExecuteQuery -> queries type: %T", assistantResponse["queries"])

				// Handle primitive.A (BSON array) type
				switch queriesVal := assistantResponse["queries"].(type) {
				case primitive.A:
					log.Printf("ChatService -> ExecuteQuery -> queries is primitive.A")
					// Convert primitive.A to []interface{}
					queries := make([]interface{}, len(queriesVal))
					for i, q := range queriesVal {
						if queryMap, ok := q.(map[string]interface{}); ok {
							// Compare hex strings of ObjectIDs
							if queryMap["id"] == query.ID.Hex() {
								queryMap["isExecuted"] = true
								queryMap["isRolledBack"] = false
								queryMap["executionTime"] = result.ExecutionTime
								queryMap["executionResult"] = map[string]interface{}{
									"result": "Query executed successfully",
								}
								if result.Error != nil {
									queryMap["error"] = map[string]interface{}{
										"code":    result.Error.Code,
										"message": result.Error.Message,
										"details": result.Error.Details,
									}
								} else {
									queryMap["error"] = nil
								}
							}
							queries[i] = queryMap
						} else {
							queries[i] = q
						}
					}
					assistantResponse["queries"] = queries

				case []interface{}:
					log.Printf("ChatService -> ExecuteQuery -> queries is []interface{}")
					for i, q := range queriesVal {
						if queryMap, ok := q.(map[string]interface{}); ok {
							if queryMap["id"] == query.ID.Hex() {
								queryMap["isExecuted"] = true
								queryMap["isRolledBack"] = false
								queryMap["executionTime"] = result.ExecutionTime
								queryMap["executionResult"] = map[string]interface{}{
									"result": "Query executed successfully",
								}
								if result.Error != nil {
									queryMap["error"] = map[string]interface{}{
										"code":    result.Error.Code,
										"message": result.Error.Message,
										"details": result.Error.Details,
									}
								} else {
									queryMap["error"] = nil
								}
								queriesVal[i] = queryMap
							}
						}
					}
					assistantResponse["queries"] = queriesVal
				}

				content["assistant_response"] = assistantResponse
			}

			// Save updated LLM message
			llmMsg.Content = content
			if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
				log.Printf("ChatService -> ExecuteQuery -> Error updating LLM message: %v", err)
			}
		}
	}()

	return &dtos.QueryExecutionResponse{
		ChatID:            chatID,
		MessageID:         msg.ID.Hex(),
		QueryID:           query.ID.Hex(),
		IsExecuted:        query.IsExecuted,
		IsRolledBack:      query.IsRolledBack,
		ExecutionTime:     query.ExecutionTime,
		ExecutionResult:   formattedResultJSON,
		Error:             result.Error,
		TotalRecordsCount: totalRecordsCount,
	}, http.StatusOK, nil
}

func (s *chatService) RollbackQuery(ctx context.Context, userID, chatID string, req *dtos.RollbackQueryRequest) (*dtos.QueryExecutionResponse, uint32, error) {
	// Verify message and query ownership
	msg, query, err := s.verifyQueryOwnership(userID, chatID, req.MessageID, req.QueryID)
	if err != nil {
		return nil, http.StatusForbidden, err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, http.StatusRequestTimeout, fmt.Errorf("query rollback cancelled or timed out")
	default:
		log.Printf("ChatService -> RollbackQuery -> msg: %+v", msg)
		log.Printf("ChatService -> RollbackQuery -> query: %+v", query)
	}

	// Validate query state
	if !query.IsExecuted {
		return nil, http.StatusBadRequest, fmt.Errorf("cannot rollback a query that hasn't been executed")
	}
	if query.IsRolledBack {
		return nil, http.StatusBadRequest, fmt.Errorf("query already rolled back")
	}

	// Check if we need to generate rollback query
	if query.RollbackQuery == nil && query.CanRollback {
		// First execute the dependent query to get context
		if query.RollbackDependentQuery == nil {
			return nil, http.StatusBadRequest, fmt.Errorf("rollback dependent query is required but not provided")
		}

		log.Printf("ChatService -> RollbackQuery -> Executing dependent query: %s", *query.RollbackDependentQuery)

		// Check connection status and connect if needed
		if !s.dbManager.IsConnected(chatID) {
			log.Printf("ChatService -> RollbackQuery -> Database not connected, initiating connection")
			status, err := s.ConnectDB(ctx, userID, chatID, req.StreamID)
			if err != nil {
				return nil, status, fmt.Errorf("failed to connect to database: %v", err)
			}
			time.Sleep(1 * time.Second)
		}

		// Execute dependent query
		dependentResult, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, *query.RollbackDependentQuery, "SELECT", false)
		if queryErr != nil {
			log.Printf("ChatService -> RollbackQuery -> queryErr: %+v", queryErr)
			if queryErr.Code == "FAILED_TO_START_TRANSACTION" || strings.Contains(queryErr.Message, "context deadline exceeded") || strings.Contains(queryErr.Message, "context canceled") {
				return nil, http.StatusRequestTimeout, fmt.Errorf("query execution timed out")
			}
			// Update query status in message
			go func() {
				if msg.Queries != nil {
					for i := range *msg.Queries {
						if (*msg.Queries)[i].ID == query.ID {
							(*msg.Queries)[i].IsExecuted = true
							(*msg.Queries)[i].IsRolledBack = false
							(*msg.Queries)[i].Error = &models.QueryError{
								Code:    queryErr.Code,
								Message: queryErr.Message,
								Details: queryErr.Details,
							}
						}
					}
				}
				if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
					log.Printf("ChatService -> RollbackQuery -> Error updating message: %v", err)
				}

				// Update LLM message with query execution results
				llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
				if err != nil {
					log.Printf("ChatService -> RollbackQuery -> Error finding LLM message: %v", err)
				} else if llmMsg != nil {
					content := llmMsg.Content
					if content == nil {
						content = make(map[string]interface{})
					}
					if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
						if queries, ok := assistantResponse["queries"].([]interface{}); ok {
							for _, q := range queries {
								if queryMap, ok := q.(map[string]interface{}); ok {
									if queryMap["id"] == query.ID.Hex() {
										queryMap["isExecuted"] = true
										queryMap["isRolledBack"] = false
										queryMap["error"] = &models.QueryError{
											Code:    queryErr.Code,
											Message: queryErr.Message,
											Details: queryErr.Details,
										}
									}
								}
							}
						}
					}

					llmMsg.Content = content
					if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
						log.Printf("ChatService -> RollbackQuery -> Error updating LLM message: %v", err)
					}
				}
			}()

			// Send event about dependent query failure
			s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
				Event: "rollback-query-failed",
				Data: map[string]interface{}{
					"chat_id":    chatID,
					"message_id": msg.ID.Hex(),
					"query_id":   query.ID.Hex(),
					"error":      queryErr,
				},
			})
			return &dtos.QueryExecutionResponse{
				ChatID:            chatID,
				MessageID:         msg.ID.Hex(),
				QueryID:           query.ID.Hex(),
				IsExecuted:        true,
				IsRolledBack:      false,
				ExecutionTime:     query.ExecutionTime,
				ExecutionResult:   nil,
				Error:             queryErr,
				TotalRecordsCount: nil,
			}, http.StatusOK, nil
		}

		// Get LLM context from previous messages
		llmMsgs, err := s.llmRepo.GetByChatID(msg.ChatID)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to get LLM context: %v", err)
		}

		// Build context for LLM
		var contextBuilder strings.Builder
		contextBuilder.WriteString("Previous messages:\n")
		for _, llmMsg := range llmMsgs {
			if content, ok := llmMsg.Content["assistant_response"].(map[string]interface{}); ok {
				contextBuilder.WriteString(fmt.Sprintf("Assistant: %v\n", content["content"]))
			}
			if content, ok := llmMsg.Content["user_message"].(string); ok {
				contextBuilder.WriteString(fmt.Sprintf("User: %s\n", content))
			}
		}
		contextBuilder.WriteString(fmt.Sprintf("\nQuery id: %s\n", query.ID.Hex())) // This will help LLM to understand the context of the query to be rolled back
		contextBuilder.WriteString(fmt.Sprintf("\nOriginal query: %s\n", query.Query))
		contextBuilder.WriteString(fmt.Sprintf("Dependent query result: %s\n", dependentResult.ResultJSON))
		contextBuilder.WriteString("\nPlease generate a rollback query that will undo the effects of the original query.")

		// Get connection info for db type
		conn, exists := s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			return nil, http.StatusBadRequest, fmt.Errorf("no database connection found")
		}

		// Convert LLM messages to expected format
		llmMessages := make([]*models.LLMMessage, len(llmMsgs))
		// Use copy to avoid modifying original messages
		copy(llmMessages, llmMsgs)

		// Get rollback query from LLM
		llmResponse, err := s.llmClient.GenerateResponse(
			ctx,
			llmMessages,      // Pass the LLM messages array
			conn.Config.Type, // Pass the database type
		)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate rollback query: %v", err)
		}

		// Parse LLM response to get rollback query
		var rollbackQuery string
		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(llmResponse), &jsonResponse); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to parse LLM response: %v", err)
		}

		if msg.Queries != nil {
			for i := range *msg.Queries {
				if (*msg.Queries)[i].ID == query.ID {
					(*msg.Queries)[i].IsExecuted = true
					(*msg.Queries)[i].IsRolledBack = false
					(*msg.Queries)[i].RollbackQuery = &rollbackQuery
				}
			}
		}
		if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
			log.Printf("ChatService -> RollbackQuery -> Error updating message: %v", err)
		}

		if assistantResponse, ok := jsonResponse["assistant_response"].(map[string]interface{}); ok {
			switch v := assistantResponse["queries"].(type) {
			case primitive.A:
				for i, q := range v {
					if qMap, ok := q.(map[string]interface{}); ok {
						if qMap["id"] == query.ID.Hex() {
							rollbackQuery = qMap["rollback_query"].(string)
							// Update the query map with rollback info
							qMap["rollback_query"] = rollbackQuery
							v[i] = qMap
						}
					}
				}
				// Update the queries in assistant response
				assistantResponse["queries"] = v
			case []interface{}:
				for i, q := range v {
					if qMap, ok := q.(map[string]interface{}); ok {
						if qMap["id"] == query.ID.Hex() {
							rollbackQuery = qMap["rollback_query"].(string)
							// Update the query map with rollback info
							qMap["rollback_query"] = rollbackQuery
							v[i] = qMap
						}
					}
				}
				// Update the queries in assistant response
				assistantResponse["queries"] = v
			}
		}

		if rollbackQuery == "" {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to generate valid rollback query")
		}

		// Update query with rollback query
		query.RollbackQuery = &rollbackQuery

		// Update query status in message
		if msg.Queries != nil {
			for i := range *msg.Queries {
				if (*msg.Queries)[i].ID == query.ID {
					(*msg.Queries)[i].RollbackQuery = &rollbackQuery
					(*msg.Queries)[i].IsRolledBack = false
					(*msg.Queries)[i].IsExecuted = true
				}
			}
		}
		// Update message in DB
		if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to update message with rollback query: %v", err)
		}

		// Update existing LLM message
		llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
		if err != nil {
			log.Printf("ChatService -> RollbackQuery -> Error finding LLM message: %v", err)
		} else if llmMsg != nil {
			content := llmMsg.Content
			if content == nil {
				content = make(map[string]interface{})
			}

			if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
				// Update the assistant response with new queries
				switch v := assistantResponse["queries"].(type) {
				case primitive.A:
					for i, q := range v {
						if qMap, ok := q.(map[string]interface{}); ok {
							if qMap["id"] == query.ID.Hex() {
								qMap["isRolledBack"] = true
								qMap["rollback_query"] = rollbackQuery
								v[i] = qMap
							}
						}
					}
				case []interface{}:
					for i, q := range v {
						if qMap, ok := q.(map[string]interface{}); ok {
							if qMap["id"] == query.ID.Hex() {
								qMap["rollback_query"] = rollbackQuery
								v[i] = qMap
							}
						}
					}
					assistantResponse["queries"] = v
				}

				content["assistant_response"] = assistantResponse
			}

			llmMsg.Content = content
			if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
				log.Printf("ChatService -> RollbackQuery -> Error updating LLM message: %v", err)
			}
		}
	}

	// Now execute the rollback query
	if query.RollbackQuery == nil {
		// Send event about rollback query failure
		s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
			Event: "rollback-query-failed",
			Data: map[string]interface{}{
				"chat_id":    chatID,
				"query_id":   query.ID.Hex(),
				"message_id": msg.ID.Hex(),
				"error":      "No rollback query available",
			},
		})
		return nil, http.StatusBadRequest, fmt.Errorf("no rollback query available")
	}

	// Check connection status and connect if needed
	if !s.dbManager.IsConnected(chatID) {
		log.Printf("ChatService -> RollbackQuery -> Database not connected, initiating connection")
		status, err := s.ConnectDB(ctx, userID, chatID, req.StreamID)
		if err != nil {
			return nil, status, fmt.Errorf("failed to connect to database: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	// Execute rollback query
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, req.MessageID, req.QueryID, req.StreamID, *query.RollbackQuery, "DML", true)
	if queryErr != nil {
		log.Printf("ChatService -> RollbackQuery -> queryErr: %+v", queryErr)
		if queryErr.Code == "FAILED_TO_START_TRANSACTION" || strings.Contains(queryErr.Message, "context deadline exceeded") || strings.Contains(queryErr.Message, "context canceled") {
			return nil, http.StatusRequestTimeout, fmt.Errorf("query execution timed out")
		}
		// Update query status in message
		go func() {
			if msg.Queries != nil {
				for i := range *msg.Queries {
					if (*msg.Queries)[i].ID == query.ID {
						(*msg.Queries)[i].IsExecuted = true
						(*msg.Queries)[i].IsRolledBack = false
						(*msg.Queries)[i].Error = &models.QueryError{
							Code:    queryErr.Code,
							Message: queryErr.Message,
							Details: queryErr.Details,
						}
					}
				}
				if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
					log.Printf("ChatService -> RollbackQuery -> Error updating message: %v", err)
				}

				// Update LLM message with query execution results
				llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
				if err != nil {
					log.Printf("ChatService -> RollbackQuery -> Error finding LLM message: %v", err)
				} else if llmMsg != nil {
					content := llmMsg.Content
					if content == nil {
						content = make(map[string]interface{})
					}
					if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
						switch v := assistantResponse["queries"].(type) {
						case primitive.A:
							for _, q := range v {
								if qMap, ok := q.(map[string]interface{}); ok {
									if qMap["id"] == query.ID.Hex() {
										qMap["isExecuted"] = true
										qMap["isRolledBack"] = false
										qMap["error"] = &models.QueryError{
											Code:    queryErr.Code,
											Message: queryErr.Message,
											Details: queryErr.Details,
										}
									}
								}
							}
							assistantResponse["queries"] = v
						case []interface{}:
							for _, q := range v {
								if qMap, ok := q.(map[string]interface{}); ok {
									if qMap["id"] == query.ID.Hex() {
										qMap["isExecuted"] = true
										qMap["isRolledBack"] = false
										if queryErr.Code != "" {
											qMap["error"] = &models.QueryError{
												Code:    queryErr.Code,
												Message: queryErr.Message,
												Details: queryErr.Details,
											}
										} else {
											qMap["error"] = nil
										}
									}
								}
							}
							assistantResponse["queries"] = v
						}
					}
					llmMsg.Content = content
					if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
						log.Printf("ChatService -> RollbackQuery -> Error updating LLM message: %v", err)
					}
				}
			}
		}()

		// Send event about rollback query failure
		s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
			Event: "rollback-query-failed",
			Data: map[string]interface{}{
				"chat_id":    chatID,
				"query_id":   query.ID.Hex(),
				"message_id": msg.ID.Hex(),
				"error":      queryErr,
			},
		})
		return &dtos.QueryExecutionResponse{
			ChatID:            chatID,
			MessageID:         msg.ID.Hex(),
			QueryID:           query.ID.Hex(),
			IsExecuted:        true,
			IsRolledBack:      false,
			ExecutionTime:     query.ExecutionTime,
			ExecutionResult:   nil,
			Error:             queryErr,
			TotalRecordsCount: nil,
		}, http.StatusOK, nil
	}

	log.Printf("ChatService -> RollbackQuery -> result: %+v", result)

	// Update query status
	// We're using same execution time for the rollback as the original query
	query.IsRolledBack = true
	query.ExecutionTime = &result.ExecutionTime
	if result.Error != nil {
		query.Error = &models.QueryError{
			Code:    result.Error.Code,
			Message: result.Error.Message,
			Details: result.Error.Details,
		}
	} else {
		query.Error = nil
	}

	// Update query status in message
	if msg.Queries != nil {
		for i := range *msg.Queries {
			if (*msg.Queries)[i].ID == query.ID {
				(*msg.Queries)[i].IsRolledBack = true
				(*msg.Queries)[i].IsExecuted = true
				(*msg.Queries)[i].ExecutionTime = &result.ExecutionTime
				(*msg.Queries)[i].ExecutionResult = &result.ResultJSON
				if result.Error != nil {
					(*msg.Queries)[i].Error = &models.QueryError{
						Code:    result.Error.Code,
						Message: result.Error.Message,
						Details: result.Error.Details,
					}
				} else {
					(*msg.Queries)[i].Error = nil
				}
			}
		}
	}
	// Save updated message
	if err := s.chatRepo.UpdateMessage(msg.ID, msg); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update message with rollback results: %v", err)
	}

	// Update LLM message with rollback results
	llmMsg, err := s.llmRepo.FindMessageByChatMessageID(msg.ID)
	if err != nil {
		log.Printf("ChatService -> RollbackQuery -> Error finding LLM message: %v", err)
	} else if llmMsg != nil {
		content := llmMsg.Content
		if content == nil {
			content = make(map[string]interface{})
		}

		if assistantResponse, ok := content["assistant_response"].(map[string]interface{}); ok {
			log.Printf("ChatService -> RollbackQuery -> assistantResponse: %+v", assistantResponse)
			log.Printf("ChatService -> RollbackQuery -> queries type: %T", assistantResponse["queries"])

			// Handle primitive.A (BSON array) type
			switch queriesVal := assistantResponse["queries"].(type) {
			case primitive.A:
				log.Printf("ChatService -> RollbackQuery -> queries is primitive.A")
				// Convert primitive.A to []interface{}
				queries := make([]interface{}, len(queriesVal))
				for i, q := range queriesVal {
					if queryMap, ok := q.(map[string]interface{}); ok {
						// Compare hex strings of ObjectIDs
						if queryMap["id"] == query.ID.Hex() {
							queryMap["isExecuted"] = true
							queryMap["isRolledBack"] = true
							queryMap["executionTime"] = result.ExecutionTime
							queryMap["executionResult"] = map[string]interface{}{
								"result": "Rolled back successfully",
							}
							if result.Error != nil {
								queryMap["error"] = map[string]interface{}{
									"code":    result.Error.Code,
									"message": result.Error.Message,
									"details": result.Error.Details,
								}
							} else {
								queryMap["error"] = nil
							}
						}
						queries[i] = queryMap
					} else {
						queries[i] = q
					}
				}
				assistantResponse["queries"] = queries

			case []interface{}:
				log.Printf("ChatService -> RollbackQuery -> queries is []interface{}")
				for i, q := range queriesVal {
					if queryMap, ok := q.(map[string]interface{}); ok {
						if queryMap["id"] == query.ID.Hex() {
							queryMap["isExecuted"] = true
							queryMap["isRolledBack"] = true
							queryMap["executionTime"] = result.ExecutionTime
							queryMap["executionResult"] = map[string]interface{}{
								"result": "Rolled back successfully",
							}
							if result.Error != nil {
								queryMap["error"] = map[string]interface{}{
									"code":    result.Error.Code,
									"message": result.Error.Message,
									"details": result.Error.Details,
								}
							} else {
								queryMap["error"] = nil
							}
							queriesVal[i] = queryMap
						}
					}
				}
				assistantResponse["queries"] = queriesVal
			}

			content["assistant_response"] = assistantResponse
		}

		// Save updated LLM message
		llmMsg.Content = content
		if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
			log.Printf("ChatService -> ExecuteQuery -> Error updating LLM message: %v", err)
		}
	}

	// Send stream event
	s.sendStreamEvent(userID, chatID, req.StreamID, dtos.StreamResponse{
		Event: "rollback-executed",
		Data: map[string]interface{}{
			"chat_id":          chatID,
			"message_id":       msg.ID.Hex(),
			"query_id":         query.ID.Hex(),
			"is_executed":      query.IsExecuted,
			"is_rolled_back":   query.IsRolledBack,
			"execution_time":   query.ExecutionTime,
			"execution_result": result.Result,
			"error":            query.Error,
		},
	})

	return &dtos.QueryExecutionResponse{
		ChatID:          chatID,
		MessageID:       msg.ID.Hex(),
		QueryID:         query.ID.Hex(),
		IsExecuted:      query.IsExecuted,
		IsRolledBack:    query.IsRolledBack,
		ExecutionTime:   query.ExecutionTime,
		ExecutionResult: result.Result,
		Error:           result.Error,
	}, http.StatusOK, nil
}

func (s *chatService) CancelQueryExecution(userID, chatID, messageID, queryID, streamID string) {
	log.Printf("ChatService -> CancelQueryExecution -> Cancelling query for streamID: %s", streamID)

	// 1. Cancel the query execution in dbManager
	s.dbManager.CancelQueryExecution(streamID)

	// 2. Send cancellation event to client
	s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
		Event: "query-cancelled",
		Data: map[string]interface{}{
			"chat_id":    chatID,
			"message_id": messageID,
			"query_id":   queryID,
			"stream_id":  streamID,
			"error": map[string]string{
				"code":    "QUERY_EXECUTION_CANCELLED",
				"message": "Query execution was cancelled by user",
			},
		},
	})

	log.Printf("ChatService -> CancelQueryExecution -> Query cancelled successfully for streamID: %s", streamID)
}

// Add helper method to verify query ownership
func (s *chatService) verifyQueryOwnership(_, chatID, messageID, queryID string) (*models.Message, *models.Query, error) {
	// Convert IDs to ObjectIDs
	msgObjID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid message ID format")
	}

	queryObjID, err := primitive.ObjectIDFromHex(queryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid query ID format")
	}

	// Get message
	msg, err := s.chatRepo.FindMessageByID(msgObjID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch message: %v", err)
	}
	if msg == nil {
		return nil, nil, fmt.Errorf("message not found")
	}

	// Verify chat ownership
	if msg.ChatID.Hex() != chatID {
		return nil, nil, fmt.Errorf("message does not belong to this chat")
	}

	log.Printf("ChatService -> verifyQueryOwnership -> msgObjID: %+v", msgObjID)
	log.Printf("ChatService -> verifyQueryOwnership -> queryObjID: %+v", queryObjID)
	log.Printf("ChatService -> verifyQueryOwnership -> msg.ChatID: %+v", msg.ChatID)

	log.Printf("ChatService -> verifyQueryOwnership -> msg: %+v", msg)
	// Find query in message
	var targetQuery *models.Query
	if msg.Queries != nil {
		for _, q := range *msg.Queries {
			if q.ID == queryObjID {
				targetQuery = &q
				break
			}
		}
	}
	if targetQuery == nil {
		return nil, nil, fmt.Errorf("query not found in message")
	}

	return msg, targetQuery, nil
}

// ProcessLLMResponseAndRunQuery processes the LLM response & runs the query automatically, updates SSE stream
func (s *chatService) ProcessLLMResponseAndRunQuery(ctx context.Context, userID, chatID string, messageID, streamID string) error {
	msgCtx, cancel := context.WithCancel(context.Background())

	log.Printf("ProcessLLMResponseAndRunQuery -> userID: %s, chatID: %s, streamID: %s", userID, chatID, streamID)

	s.processesMu.Lock()
	s.activeProcesses[streamID] = cancel
	s.processesMu.Unlock()

	// Use the parent context (ctx) for SSE connection
	// Use llmCtx for LLM processing
	go func() {
		defer func() {
			s.processesMu.Lock()
			delete(s.activeProcesses, streamID)
			s.processesMu.Unlock()
		}()

		msgResp, err := s.processLLMResponse(msgCtx, userID, chatID, streamID, true, true)
		if err != nil {
			log.Printf("Error processing LLM response: %v", err)
			return
		}
		log.Printf("ProcessLLMResponseAndRunQuery -> msgResp: %v", msgResp)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		select {
		case <-ctx.Done():
			log.Printf("Query execution timed out")
			return
		default:
			log.Printf("ProcessLLMResponseAndRunQuery -> msgResp.Queries: %v", msgResp.Queries)
			if msgResp.Queries != nil {
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-step",
					Data:  "Executing the needful query now.",
				})
				tempQueries := make([]dtos.Query, len(*msgResp.Queries))
				for i, query := range *msgResp.Queries {
					if query.Query != "" && !query.IsCritical {
						executionResult, _, queryErr := s.ExecuteQuery(ctx, userID, chatID, &dtos.ExecuteQueryRequest{
							MessageID: msgResp.ID,
							QueryID:   query.ID,
							StreamID:  streamID,
						})
						if queryErr != nil {
							log.Printf("Error executing query: %v", queryErr)
							// Send existing msgResp so far
							s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
								Event: "ai-response",
								Data:  msgResp,
							})
							return
						}
						log.Printf("ProcessLLMResponseAndRunQuery -> Query executed successfully: %v", executionResult)
						query.IsExecuted = true
						query.ExecutionTime = executionResult.ExecutionTime
						query.ExecutionResult = executionResult.ExecutionResult.(map[string]interface{})
						query.Error = executionResult.Error
						if query.Pagination != nil && executionResult.TotalRecordsCount != nil {
							query.Pagination.TotalRecordsCount = *executionResult.TotalRecordsCount
						}
					}
					tempQueries[i] = query
				}
				msgResp.Queries = &tempQueries
				log.Printf("ProcessLLMResponseAndRunQuery -> Queries updated in LLM response: %v", msgResp.Queries)
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response",
					Data:  msgResp,
				})
				return
			} else {
				log.Printf("No queries found in LLM response, returning ai response")
				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response",
					Data:  msgResp,
				})
				return
			}
		}
	}()
	return nil
}

// Update the ProcessMessage method to use a separate context for LLM processing
func (s *chatService) ProcessMessage(_ context.Context, userID, chatID string, streamID string) error {
	// Create a new context specifically for LLM processing
	// Use context.Background() to avoid cancellation of the parent context
	msgCtx, cancel := context.WithCancel(context.Background())

	log.Printf("ProcessMessage -> userID: %s, chatID: %s, streamID: %s", userID, chatID, streamID)

	s.processesMu.Lock()
	s.activeProcesses[streamID] = cancel
	s.processesMu.Unlock()

	// Use the parent context (ctx) for SSE connection
	// Use llmCtx for LLM processing
	go func() {
		defer func() {
			s.processesMu.Lock()
			delete(s.activeProcesses, streamID)
			s.processesMu.Unlock()
		}()

		if err := s.processMessageInternal(msgCtx, userID, chatID, streamID); err != nil {
			log.Printf("Error processing message: %v", err)
			// Use parent context for sending stream events
			select {
			case <-msgCtx.Done():
				return
			default:
				go func() {
					// Get user and chat IDs
					userObjID, cErr := primitive.ObjectIDFromHex(userID)
					if cErr != nil {
						log.Printf("Error processing message: %v", cErr)
						return
					}

					chatObjID, cErr := primitive.ObjectIDFromHex(chatID)
					if cErr != nil {
						log.Printf("Error processing message: %v", err)
						return
					}

					// Create a new error message
					errorMsg := &models.Message{
						Base:    models.NewBase(),
						UserID:  userObjID,
						ChatID:  chatObjID,
						Queries: nil,
						Content: "Error: " + err.Error(),
						Type:    string(MessageTypeAssistant),
					}

					if err := s.chatRepo.CreateMessage(errorMsg); err != nil {
						log.Printf("Error processing message: %v", err)
						return
					}
				}()

				s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
					Event: "ai-response-error",
					Data:  "Error: " + err.Error(),
				})
			}
		}
	}()

	return nil
}

// Update processMessageInternal to use both contexts
func (s *chatService) processMessageInternal(msgCtx context.Context, userID, chatID, streamID string) error {
	// Cancellation with s.activeProcesses[streamID]
	log.Printf("processMessageInternal -> userID: %s, chatID: %s, streamID: %s", userID, chatID, streamID)
	select {
	case <-msgCtx.Done():
		return fmt.Errorf("sse connection closed")
	default:
		// LLM processing will be handled in this method
		s.processLLMResponse(msgCtx, userID, chatID, streamID, false, true)
	}

	return nil
}

func (s *chatService) RefreshSchema(ctx context.Context, userID, chatID string, sync bool) (uint32, error) {
	log.Printf("ChatService -> RefreshSchema -> Starting for chatID: %s", chatID)

	// Increase the timeout for the initial context to 60 minutes
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return http.StatusOK, nil
	default:
		// Check if connection exists
		_, exists := s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			log.Printf("ChatService -> RefreshSchema -> Connection not found for chatID: %s", chatID)
			return http.StatusNotFound, fmt.Errorf("connection not found")
		}

		// Get chat to get selected collections
		chatObjID, err := primitive.ObjectIDFromHex(chatID)
		if err != nil {
			log.Printf("ChatService -> RefreshSchema -> Error getting chatID: %v", err)
			return http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
		}

		chat, err := s.chatRepo.FindByID(chatObjID)
		if err != nil {
			log.Printf("ChatService -> RefreshSchema -> Error finding chat: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
		}

		if chat == nil {
			log.Printf("ChatService -> RefreshSchema -> Chat not found for chatID: %s", chatID)
			return http.StatusNotFound, fmt.Errorf("chat not found")
		}

		// Convert the selectedCollections string to a slice
		var selectedCollectionsSlice []string
		if chat.SelectedCollections != "ALL" && chat.SelectedCollections != "" {
			selectedCollectionsSlice = strings.Split(chat.SelectedCollections, ",")
		}
		log.Printf("ChatService -> RefreshSchema -> Selected collections: %v", selectedCollectionsSlice)

		dataChan := make(chan error, 1)
		go func() {
			// Create a new context with a longer timeout specifically for the schema refresh operation
			// Increase to 90 minutes to handle large schemas or slow database responses
			schemaCtx, schemaCancel := context.WithTimeout(context.Background(), 90*time.Minute)
			defer schemaCancel()

			userObjID, err := primitive.ObjectIDFromHex(userID)
			if err != nil {
				log.Printf("ChatService -> RefreshSchema -> Error getting userID: %v", err)
				dataChan <- err
				return
			}

			// Force a fresh schema fetch by using a new context with a longer timeout
			log.Printf("ChatService -> RefreshSchema -> Forcing fresh schema fetch for chatID: %s with 90-minute timeout", chatID)

			// Use the method to get schema with examples and pass selected collections
			schemaMsg, err := s.dbManager.RefreshSchemaWithExamples(schemaCtx, chatID, selectedCollectionsSlice)
			if err != nil {
				log.Printf("ChatService -> RefreshSchema -> Error refreshing schema with examples: %v", err)
				dataChan <- err
				return
			}

			if schemaMsg == "" {
				log.Printf("ChatService -> RefreshSchema -> Warning: Empty schema message returned")
				schemaMsg = "Schema refresh completed, but no schema information was returned. Please check your database connection and selected tables."
			}

			log.Printf("ChatService -> RefreshSchema -> schemaMsg length: %d", len(schemaMsg))
			llmMsg := &models.LLMMessage{
				Base:   models.NewBase(),
				UserID: userObjID,
				ChatID: chatObjID,
				Role:   string(MessageTypeSystem),
				Content: map[string]interface{}{
					"schema_update": schemaMsg,
				},
			}

			// Clear previous system message from LLM
			if err := s.llmRepo.DeleteMessagesByRole(chatObjID, string(MessageTypeSystem)); err != nil {
				log.Printf("ChatService -> RefreshSchema -> Error deleting system message: %v", err)
			}

			if err := s.llmRepo.CreateMessage(llmMsg); err != nil {
				log.Printf("ChatService -> RefreshSchema -> Error saving LLM message: %v", err)
			}
			log.Println("ChatService -> RefreshSchema -> Schema refreshed successfully")
			dataChan <- nil // Will be used to Synchronous refresh
		}()

		if sync {
			log.Println("ChatService -> RefreshSchema -> Waiting for Synchronous refresh to complete")
			<-dataChan
			log.Println("ChatService -> RefreshSchema -> Synchronous refresh completed")
		}
		return http.StatusOK, nil
	}
}

// Fetches paginated results for a query
func (s *chatService) GetQueryResults(ctx context.Context, userID, chatID, messageID, queryID, streamID string, offset int) (*dtos.QueryResultsResponse, uint32, error) {
	log.Printf("ChatService -> GetQueryResults -> userID: %s, chatID: %s, messageID: %s, queryID: %s, streamID: %s, offset: %d", userID, chatID, messageID, queryID, streamID, offset)
	_, query, err := s.verifyQueryOwnership(userID, chatID, messageID, queryID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	if query.Pagination == nil {
		return nil, http.StatusBadRequest, fmt.Errorf("query does not support pagination")
	}
	if query.Pagination.PaginatedQuery == nil {
		return nil, http.StatusBadRequest, fmt.Errorf("query does not support pagination")
	}

	// Check the connection status and connect if needed
	if !s.dbManager.IsConnected(chatID) {
		status, err := s.ConnectDB(ctx, userID, chatID, streamID)
		if err != nil {
			return nil, status, fmt.Errorf("failed to connect to database: %v", err)
		}
	}
	log.Printf("ChatService -> GetQueryResults -> query.Pagination.PaginatedQuery: %+v", query.Pagination.PaginatedQuery)
	offSettPaginatedQuery := strings.Replace(*query.Pagination.PaginatedQuery, "offset_size", strconv.Itoa(offset), 1)
	log.Printf("ChatService -> GetQueryResults -> offSettPaginatedQuery: %+v", offSettPaginatedQuery)
	result, queryErr := s.dbManager.ExecuteQuery(ctx, chatID, messageID, queryID, streamID, offSettPaginatedQuery, *query.QueryType, false)
	if queryErr != nil {
		log.Printf("ChatService -> GetQueryResults -> queryErr: %+v", queryErr)
		return nil, http.StatusBadRequest, fmt.Errorf(queryErr.Message)
	}

	log.Printf("ChatService -> GetQueryResults -> result: %+v", result)
	log.Printf("ChatService -> GetQueryResults -> result.ResultJSON: %+v", result.ResultJSON)

	var formattedResultJSON interface{}
	var resultListFormatting []interface{} = []interface{}{}
	var resultMapFormatting map[string]interface{} = map[string]interface{}{}
	if err := json.Unmarshal([]byte(result.ResultJSON), &resultListFormatting); err != nil {
		if err := json.Unmarshal([]byte(result.ResultJSON), &resultMapFormatting); err != nil {
			log.Printf("ChatService -> GetQueryResults -> Error unmarshalling result JSON: %v", err)
			// Try to unmarshal as a map
			err = json.Unmarshal([]byte(result.ResultJSON), &resultMapFormatting)
			if err != nil {
				log.Printf("ChatService -> GetQueryResults -> Error unmarshalling result JSON: %v", err)
			}
		}
	}

	if len(resultListFormatting) > 0 {
		formattedResultJSON = resultListFormatting
	} else {
		formattedResultJSON = resultMapFormatting
	}

	log.Printf("ChatService -> GetQueryResults -> formattedResultJSON: %+v", formattedResultJSON)

	s.sendStreamEvent(userID, chatID, streamID, dtos.StreamResponse{
		Event: "query-paginated-results",
		Data: map[string]interface{}{
			"chat_id":             chatID,
			"message_id":          messageID,
			"query_id":            queryID,
			"execution_result":    formattedResultJSON,
			"error":               queryErr,
			"total_records_count": query.Pagination.TotalRecordsCount,
		},
	})
	return &dtos.QueryResultsResponse{
		ChatID:            chatID,
		MessageID:         messageID,
		QueryID:           queryID,
		ExecutionResult:   formattedResultJSON,
		Error:             queryErr,
		TotalRecordsCount: query.Pagination.TotalRecordsCount,
	}, http.StatusOK, nil
}

func (s *chatService) EditQuery(ctx context.Context, userID, chatID, messageID, queryID string, query string) (*dtos.EditQueryResponse, uint32, error) {
	log.Printf("ChatService -> EditQuery -> userID: %s, chatID: %s, messageID: %s, queryID: %s, query: %s", userID, chatID, messageID, queryID, query)

	message, queryData, err := s.verifyQueryOwnership(userID, chatID, messageID, queryID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	if queryData.IsExecuted || queryData.IsRolledBack {
		return nil, http.StatusBadRequest, fmt.Errorf("query has already been executed, cannot edit")
	}

	originalQuery := queryData.Query
	// Fix the query update logic
	for i := range *message.Queries {
		if (*message.Queries)[i].ID == queryData.ID {
			(*message.Queries)[i].Query = query
			(*message.Queries)[i].IsEdited = true
			if (*message.Queries)[i].Pagination != nil && (*message.Queries)[i].Pagination.PaginatedQuery != nil {
				(*message.Queries)[i].Pagination.PaginatedQuery = utils.ToStringPtr(strings.Replace(*(*message.Queries)[i].Pagination.PaginatedQuery, originalQuery, query, 1))
			}
		}
	}

	message.IsEdited = true
	if err := s.chatRepo.UpdateMessage(message.ID, message); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to update message: %v", err)
	}

	// Update the query in LLM messages too
	llmMsg, err := s.llmRepo.FindMessageByChatMessageID(message.ID)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to find LLM message: %v", err)
	}

	if assistantResponse, ok := llmMsg.Content["assistant_response"].(map[string]interface{}); ok {
		log.Printf("ChatService -> EditQuery -> assistantResponse: %+v", assistantResponse)
		log.Printf("ChatService -> EditQuery -> queries type: %T", assistantResponse["queries"])

		llmMsg.IsEdited = true
		queries := assistantResponse["queries"]
		// Handle primitive.A (BSON array) type
		switch queriesVal := queries.(type) {
		case primitive.A:
			for i, q := range queriesVal {
				qMap, ok := q.(map[string]interface{})
				if !ok {
					continue
				}
				if qMap["id"] == queryData.ID {
					qMap["query"] = "EDITED by user: " + query // Telling the LLM that the query has been edited
					qMap["is_edited"] = true
					qMap["is_executed"] = false
					if qMap["pagination"] != nil {
						currentPaginatedQuery := qMap["pagination"].(map[string]interface{})["paginated_query"].(string)
						qMap["pagination"].(map[string]interface{})["paginated_query"] = utils.ToStringPtr(strings.Replace(currentPaginatedQuery, originalQuery, query, 1))
					}
					queriesVal[i] = qMap
					break
				}
			}
			assistantResponse["queries"] = queriesVal
		case []interface{}:
			for i, q := range queriesVal {
				qMap, ok := q.(map[string]interface{})
				if !ok {
					continue
				}
				if qMap["id"] == queryData.ID {
					qMap["query"] = "EDITED by user: " + query // Telling the LLM that the query has been edited
					qMap["is_edited"] = true
					qMap["is_executed"] = false
					if qMap["pagination"] != nil {
						currentPaginatedQuery := qMap["pagination"].(map[string]interface{})["paginated_query"].(string)
						qMap["pagination"].(map[string]interface{})["paginated_query"] = utils.ToStringPtr(strings.Replace(currentPaginatedQuery, originalQuery, query, 1))
					}
					queriesVal[i] = qMap
					break
				}
			}
			assistantResponse["queries"] = queriesVal
		}
	}

	if err := s.llmRepo.UpdateMessage(llmMsg.ID, llmMsg); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to update LLM message: %v", err)
	}

	return &dtos.EditQueryResponse{
		ChatID:    chatID,
		MessageID: messageID,
		QueryID:   queryID,
		Query:     query,
		IsEdited:  true,
	}, http.StatusOK, nil
}

// New method for getting tables
func (s *chatService) GetTables(ctx context.Context, userID, chatID string) (*dtos.TablesResponse, uint32, error) {
	return s.GetAllTables(ctx, userID, chatID)
}

// GetSelectedCollections retrieves the selected collections for a chat
func (s *chatService) GetSelectedCollections(chatID string) (string, error) {
	log.Printf("ChatService -> GetSelectedCollections -> Starting for chatID: %s", chatID)

	// Convert to ObjectID
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		log.Printf("ChatService -> GetSelectedCollections -> Error getting chatID: %v", err)
		return "ALL", fmt.Errorf("invalid chat ID format: %v", err)
	}

	// Get chat to get selected collections
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		log.Printf("ChatService -> GetSelectedCollections -> Error finding chat: %v", err)
		return "ALL", fmt.Errorf("failed to fetch chat: %v", err)
	}

	if chat == nil {
		log.Printf("ChatService -> GetSelectedCollections -> Chat not found for chatID: %s", chatID)
		return "ALL", fmt.Errorf("chat not found")
	}

	log.Printf("ChatService -> GetSelectedCollections -> Selected collections for chatID %s: %s", chatID, chat.SelectedCollections)

	// If SelectedCollections is empty, return "ALL"
	if chat.SelectedCollections == "" {
		return "ALL", nil
	}

	return chat.SelectedCollections, nil
}

// New method for getting all tables (for UI display)
func (s *chatService) GetAllTables(ctx context.Context, userID, chatID string) (*dtos.TablesResponse, uint32, error) {
	log.Printf("ChatService -> GetAllTables -> Starting for chatID: %s", chatID)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, http.StatusRequestTimeout, fmt.Errorf("request timed out")
	default:
		// Get chat details first
		chatObjID, err := primitive.ObjectIDFromHex(chatID)
		if err != nil {
			log.Printf("ChatService -> GetAllTables -> Error getting chatID: %v", err)
			return nil, http.StatusBadRequest, fmt.Errorf("invalid chat ID format")
		}

		chat, err := s.chatRepo.FindByID(chatObjID)
		if err != nil {
			log.Printf("ChatService -> GetAllTables -> Error finding chat: %v", err)
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to fetch chat: %v", err)
		}

		if chat == nil {
			log.Printf("ChatService -> GetAllTables -> Chat not found for chatID: %s", chatID)
			return nil, http.StatusNotFound, fmt.Errorf("chat not found")
		}

		// Get database connection
		dbConn, err := s.dbManager.GetConnection(chatID)
		if err != nil {
			log.Printf("ChatService -> GetAllTables -> Connection not found, attempting to connect: %v", err)

			// Connection not found, try to connect with proper config
			connectErr := s.dbManager.Connect(chatID, userID, "", dbmanager.ConnectionConfig{
				Type:     chat.Connection.Type,
				Host:     chat.Connection.Host,
				Port:     chat.Connection.Port,
				Username: chat.Connection.Username,
				Password: chat.Connection.Password,
				Database: chat.Connection.Database,
			})
			if connectErr != nil {
				log.Printf("ChatService -> GetAllTables -> Failed to connect: %v", connectErr)
				return nil, http.StatusNotFound, fmt.Errorf("failed to establish database connection: %v", connectErr)
			}

			// Try to get connection again after connecting
			dbConn, err = s.dbManager.GetConnection(chatID)
			if err != nil {
				log.Printf("ChatService -> GetAllTables -> Still failed to get connection after connect: %v", err)
				return nil, http.StatusNotFound, fmt.Errorf("connection established but not ready yet: %v", err)
			}
		}

		// Get connection info
		connInfo, exists := s.dbManager.GetConnectionInfo(chatID)
		if !exists {
			log.Printf("ChatService -> GetAllTables -> Connection info not found")
			return nil, http.StatusNotFound, fmt.Errorf("connection info not found")
		}

		// Convert the selectedCollections string to a slice
		var selectedCollectionsSlice []string
		if chat.SelectedCollections != "ALL" && chat.SelectedCollections != "" {
			selectedCollectionsSlice = strings.Split(chat.SelectedCollections, ",")
		}
		log.Printf("ChatService -> GetAllTables -> Selected collections: %v", selectedCollectionsSlice)

		// Create a map for quick lookup of selected tables
		selectedTablesMap := make(map[string]bool)
		for _, tableName := range selectedCollectionsSlice {
			selectedTablesMap[tableName] = true
		}
		isAllSelected := chat.SelectedCollections == "ALL" || chat.SelectedCollections == ""

		// Get schema manager
		schemaManager := s.dbManager.GetSchemaManager()

		// Get schema from database - pass empty slice to get ALL tables
		schema, err := schemaManager.GetSchema(ctx, chatID, dbConn, connInfo.Config.Type, []string{})
		if err != nil {
			log.Printf("ChatService -> GetAllTables -> Error getting schema: %v", err)
			return nil, http.StatusInternalServerError, fmt.Errorf("failed to get schema: %v", err)
		}

		// Convert schema tables to TableInfo objects
		var tables []dtos.TableInfo
		for tableName, tableSchema := range schema.Tables {
			tableInfo := dtos.TableInfo{
				Name:       tableName,
				Columns:    make([]dtos.ColumnInfo, 0, len(tableSchema.Columns)),
				IsSelected: isAllSelected || selectedTablesMap[tableName],
			}

			for columnName, columnInfo := range tableSchema.Columns {
				tableInfo.Columns = append(tableInfo.Columns, dtos.ColumnInfo{
					Name:       columnName,
					Type:       columnInfo.Type,
					IsNullable: columnInfo.IsNullable,
				})
			}

			tables = append(tables, tableInfo)
		}

		// Sort tables by name for consistent output
		sort.Slice(tables, func(i, j int) bool {
			return tables[i].Name < tables[j].Name
		})

		return &dtos.TablesResponse{
			Tables: tables,
		}, http.StatusOK, nil
	}
}
