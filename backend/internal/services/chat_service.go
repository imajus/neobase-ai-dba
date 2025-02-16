package services

import (
	"fmt"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/models"
	"neobase-ai/internal/repositories"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatService interface {
	Create(userID string, req *dtos.CreateChatRequest) (*dtos.ChatResponse, uint32, error)
	Update(userID, chatID string, req *dtos.UpdateChatRequest) (*dtos.ChatResponse, uint32, error)
	Delete(userID, chatID string) (uint32, error)
	GetByID(userID, chatID string) (*dtos.ChatResponse, uint32, error)
	List(userID string, page, pageSize int) (*dtos.ChatListResponse, uint32, error)
	CreateMessage(userID, chatID string, req *dtos.CreateMessageRequest) (*dtos.MessageResponse, uint32, error)
	UpdateMessage(userID, chatID, messageID string, req *dtos.CreateMessageRequest) (*dtos.MessageResponse, uint32, error)
	DeleteMessages(userID, chatID string) (uint32, error)
	ListMessages(userID, chatID string, page, pageSize int) (*dtos.MessageListResponse, uint32, error)
	StreamResponse(userID, chatID, streamID string) (chan dtos.StreamResponse, error)
}

type chatService struct {
	chatRepo repositories.ChatRepository
	llmRepo  repositories.LLMMessageRepository
}

func NewChatService(chatRepo repositories.ChatRepository, llmRepo repositories.LLMMessageRepository) ChatService {
	return &chatService{
		chatRepo: chatRepo,
		llmRepo:  llmRepo,
	}
}

func (s *chatService) Create(userID string, req *dtos.CreateChatRequest) (*dtos.ChatResponse, uint32, error) {
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
		IsActive: true,
		Base:     models.NewBase(),
	}

	// Create chat with connection
	chat := models.NewChat(userObjID, connection)
	if err := s.chatRepo.Create(chat); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create chat: %v", err)
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

	// Update chat fields
	chat.IsActive = req.IsActive
	if req.Connection != (dtos.CreateConnectionRequest{}) {
		chat.Connection = models.Connection{
			Type:     req.Connection.Type,
			Host:     req.Connection.Host,
			Port:     req.Connection.Port,
			Username: &req.Connection.Username,
			Password: &req.Connection.Password,
			Database: req.Connection.Database,
			IsActive: true,
			Base:     models.NewBase(),
		}
	}

	if err := s.chatRepo.Update(chatObjID, chat); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to update chat: %v", err)
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
	if err := s.llmRepo.DeleteMessagesByChatID(chatObjID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete chat messages: %v", err)
	}

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

func (s *chatService) CreateMessage(userID, chatID string, req *dtos.CreateMessageRequest) (*dtos.MessageResponse, uint32, error) {
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

	// Create message
	message := models.NewMessage(userObjID, chatObjID, req.Type, req.Content, nil)
	if err := s.chatRepo.CreateMessage(message); err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create message: %v", err)
	}

	return s.buildMessageResponse(message), http.StatusCreated, nil
}

func (s *chatService) UpdateMessage(userID, chatID, messageID string, req *dtos.CreateMessageRequest) (*dtos.MessageResponse, uint32, error) {
	// Implementation similar to CreateMessage but with update logic
	return nil, http.StatusNotImplemented, fmt.Errorf("not implemented")
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

	messages, total, err := s.chatRepo.FindMessagesByChat(chatObjID, page, pageSize)
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

func (s *chatService) StreamResponse(userID, chatID, streamID string) (chan dtos.StreamResponse, error) {
	// Convert string IDs to ObjectIDs
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format")
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID format")
	}

	// Verify chat ownership and existence
	chat, err := s.chatRepo.FindByID(chatObjID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chat: %v", err)
	}
	if chat == nil {
		return nil, fmt.Errorf("chat not found")
	}
	if chat.UserID != userObjID {
		return nil, fmt.Errorf("unauthorized access to chat")
	}

	// Create channel for streaming responses
	streamChan := make(chan dtos.StreamResponse)

	// Start streaming in a goroutine
	go func() {
		defer close(streamChan)

		// TODO: Prepare the LLM Messages to send to the LLM

		// TODO: Here you would integrate with your LLM service
		// For now, we'll simulate some streaming responses
		responses := []string{
			"Analyzing your query...",
			"Generating SQL statements...",
			"Validating database schema...",
			"Preparing response...",
			"Here's what I found...",
		}

		for _, resp := range responses {
			select {
			case streamChan <- dtos.StreamResponse{
				Event: "message",
				Data:  resp,
			}:
				time.Sleep(1 * time.Second) // Simulate processing time
			case <-time.After(5 * time.Second):
				// Timeout if channel is blocked
				return
			}
		}

		// Send completion event
		streamChan <- dtos.StreamResponse{
			Event: "complete",
			Data:  map[string]interface{}{},
		}
	}()

	return streamChan, nil
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
			Password: *chat.Connection.Password,
			Database: chat.Connection.Database,
		},
		IsActive:  chat.IsActive,
		CreatedAt: chat.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: chat.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *chatService) buildMessageResponse(msg *models.Message) *dtos.MessageResponse {
	return &dtos.MessageResponse{
		ID:        msg.ID.Hex(),
		ChatID:    msg.ChatID.Hex(),
		Type:      msg.Type,
		Content:   msg.Content,
		Queries:   dtos.ToQueryDto(msg.Queries),
		CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
