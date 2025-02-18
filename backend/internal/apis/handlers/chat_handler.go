package handlers

import (
	"encoding/json"
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

// HandleStreamEvent implements the StreamHandler interface
func (h *ChatHandler) HandleStreamEvent(userID, chatID, streamID string, response dtos.StreamResponse) {
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)

	h.mu.RLock()
	streamChan, exists := h.streams[streamKey]
	h.mu.RUnlock()

	if exists {
		select {
		case streamChan <- response:
			log.Printf("Sent stream event: %s for stream %s", response.Event, streamID)
		default:
			log.Printf("Failed to send stream event: channel full or closed for stream %s", streamID)
		}
	} else {
		log.Printf("No stream channel found for key: %s", streamKey)
	}
}

// StreamChat handles SSE endpoint
func (h *ChatHandler) StreamChat(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("stream_id")

	if streamID == "" {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr("stream_id is required"),
		})
		return
	}

	// Create new stream channel
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)
	h.mu.Lock()
	streamChan := make(chan dtos.StreamResponse)
	h.streams[streamKey] = streamChan
	h.mu.Unlock()

	log.Printf("Chat SSE Connected: userID=%s, chatID=%s, streamID=%s", userID, chatID, streamID)

	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable buffering in Nginx

	// Send initial connection message
	c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n",
		`{"event":"sse-connected","data":"SSE Connected successfully"}`)))
	c.Writer.Flush()

	// Subscribe to chat service stream
	h.chatService.SetStreamHandler(h)

	// Cleanup when done
	defer func() {
		h.mu.Lock()
		if ch, exists := h.streams[streamKey]; exists {
			close(ch)
			delete(h.streams, streamKey)
		}
		h.mu.Unlock()
		log.Printf("Chat SSE Disconnected: userID=%s, chatID=%s, streamID=%s", userID, chatID, streamID)
	}()

	// Stream responses
	c.Stream(func(w io.Writer) bool {
		select {
		case response, ok := <-streamChan:
			if !ok {
				return false
			}
			// Format SSE message manually
			jsonData, err := json.Marshal(response)
			if err != nil {
				log.Printf("Error marshaling SSE response: %v", err)
				return false
			}
			// Write SSE message format
			_, err = w.Write([]byte(fmt.Sprintf("data: %s\n\n", jsonData)))
			if err != nil {
				log.Printf("Error writing SSE message: %v", err)
				return false
			}
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// CancelStream cancels currently streaming response
func (h *ChatHandler) CancelStream(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("stream_id")

	if streamID == "" {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr("stream_id is required"),
		})
		return
	}

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
	chatID := c.Param("id")

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(fmt.Sprintf("Invalid request: %v", err)),
		})
		return
	}

	statusCode, err := h.chatService.ConnectDB(c.Request.Context(), userID, chatID, req.StreamID)
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
	chatID := c.Param("id")
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(fmt.Sprintf("Invalid request: %v", err)),
		})
		return
	}

	statusCode, err := h.chatService.DisconnectDB(c.Request.Context(), userID, chatID, req.StreamID)
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

// Add query execution methods
func (h *ChatHandler) ExecuteQuery(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	var req dtos.ExecuteQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	// Execute query
	response, status, err := h.chatService.ExecuteQuery(c.Request.Context(), userID, chatID, &req)
	if err != nil {
		c.JSON(int(status), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(int(status), response)
}

func (h *ChatHandler) RollbackQuery(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")

	var req dtos.RollbackQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	// Execute rollback
	response, status, err := h.chatService.RollbackQuery(c.Request.Context(), userID, chatID, &req)
	if err != nil {
		c.JSON(int(status), dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr(err.Error()),
		})
		return
	}

	c.JSON(int(status), response)
}

func (h *ChatHandler) CancelQueryExecution(c *gin.Context) {
	var req dtos.CancelQueryExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Cancel execution
	h.chatService.CancelQueryExecution(req.StreamID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Query execution cancelled",
		"data": map[string]interface{}{
			"stream_id": req.StreamID,
		},
	})
}
