package handlers

import (
	"fmt"
	"io"
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"
	"neobase-ai/internal/utils"
	"net/http"
	"strconv"
	"sync"

	"neobase-ai/pkg/dbmanager"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService services.ChatService
	dbManager   *dbmanager.Manager
	mu          sync.RWMutex
	streams     map[string]chan dtos.StreamResponse // key: streamID
}

func NewChatHandler(chatService services.ChatService, dbManager *dbmanager.Manager) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		dbManager:   dbManager,
		streams:     make(map[string]chan dtos.StreamResponse),
	}
}

func (h *ChatHandler) Create(c *gin.Context) {
	var req dtos.CreateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	userID := c.GetString("userID")
	response, statusCode, err := h.chatService.Create(userID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

func (h *ChatHandler) List(c *gin.Context) {
	userID := c.GetString("userID")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	response, statusCode, err := h.chatService.List(userID, page, pageSize)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

func (h *ChatHandler) GetByID(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	response, statusCode, err := h.chatService.GetByID(userID, chatID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

func (h *ChatHandler) Update(c *gin.Context) {
	var req dtos.UpdateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	userID := c.GetString("userID")
	chatID := c.Param("id")

	response, statusCode, err := h.chatService.Update(userID, chatID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

func (h *ChatHandler) Delete(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	statusCode, err := h.chatService.Delete(userID, chatID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Chat deleted successfully",
	})
}

func (h *ChatHandler) ListMessages(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	response, statusCode, err := h.chatService.ListMessages(userID, chatID, page, pageSize)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

func (h *ChatHandler) CreateMessage(c *gin.Context) {
	var req dtos.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	userID := c.GetString("userID")
	chatID := c.Param("id")

	response, statusCode, err := h.chatService.CreateMessage(c.Request.Context(), userID, chatID, req.StreamID, req.Content)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

func (h *ChatHandler) UpdateMessage(c *gin.Context) {
	var req dtos.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	userID := c.GetString("userID")
	chatID := c.Param("id")
	messageID := c.Param("messageId")

	response, statusCode, err := h.chatService.UpdateMessage(userID, chatID, messageID, &req)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    response,
	})
}

func (h *ChatHandler) DeleteMessages(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	statusCode, err := h.chatService.DeleteMessages(userID, chatID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(int(statusCode), dtos.Response{
		Success: true,
		Data:    "Messages deleted successfully",
	})
}

// HandleStreamEvent receives stream events from chat service and sends them to the client
func (h *ChatHandler) HandleStreamEvent(userID, chatID, streamID string, response dtos.StreamResponse) {
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)
	h.mu.RLock()
	streamChan, exists := h.streams[streamKey]
	h.mu.RUnlock()

	if exists {
		select {
		case streamChan <- response:
		default:
			log.Printf("Warning: Stream channel %s is blocked", streamKey)
		}
	}
}

// StreamChat handles SSE endpoint
func (h *ChatHandler) StreamChat(c *gin.Context) {
	userID := c.GetString("user_id")
	chatID := c.Param("chat_id")
	streamID := c.Query("stream_id")

	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Create new stream channel
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)
	h.mu.Lock()
	streamChan := make(chan dtos.StreamResponse)
	h.streams[streamKey] = streamChan
	h.mu.Unlock()

	// Send Connected successfully
	c.SSEvent("message", dtos.StreamResponse{
		Event: "sse-connected",
		Data:  "SSE Connected successfully",
	})

	// Subscribe to chat service stream
	h.chatService.SetStreamHandler(h)

	// Cleanup when done
	defer func() {
		h.mu.Lock()
		delete(h.streams, streamKey)
		close(streamChan)
		h.mu.Unlock()
	}()

	// Stream responses
	c.Stream(func(w io.Writer) bool {
		select {
		case response, ok := <-streamChan:
			if !ok {
				return false
			}
			// Send SSE format
			c.SSEvent(response.Event, response.Data)
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// CancelStream cancels currently streaming response
func (h *ChatHandler) CancelStream(c *gin.Context) {
	userID := c.GetString("user_id")
	chatID := c.Param("chat_id")
	streamID := c.Query("stream_id")

	// Create stream key
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)

	// First cancel the processing
	h.chatService.CancelProcessing(streamID)

	// Then cleanup the stream
	h.mu.Lock()
	if streamChan, ok := h.streams[streamKey]; ok {
		close(streamChan)
		delete(h.streams, streamKey)
	}
	h.mu.Unlock()

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Operation cancelled successfully",
	})
}

// ConnectDB establishes a database connection
func (h *ChatHandler) ConnectDB(c *gin.Context) {
	var req dtos.ConnectDBRequest
	userID := c.GetString("userID")

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(fmt.Sprintf("Invalid request: %v", err)),
		})
		return
	}

	statusCode, err := h.chatService.ConnectDB(c.Request.Context(), userID, req.ChatID, req.StreamID)
	if err != nil {
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Database connected successfully",
	})
}

// DisconnectDB closes a database connection
func (h *ChatHandler) DisconnectDB(c *gin.Context) {
	var req dtos.DisconnectDBRequest
	userID := c.GetString("userID")

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(fmt.Sprintf("Invalid request: %v", err)),
		})
		return
	}

	statusCode, err := h.chatService.DisconnectDB(c.Request.Context(), userID, req.ChatID, req.StreamID)
	if err != nil {
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Database disconnected successfully",
	})
}

// GetDBConnectionStatus checks the current connection status
func (h *ChatHandler) GetDBConnectionStatus(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	status, statusCode, err := h.chatService.GetDBConnectionStatus(c.Request.Context(), userID, chatID)
	if err != nil {
		c.JSON(int(statusCode), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    status,
	})
}
