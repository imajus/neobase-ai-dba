package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"
	"neobase-ai/internal/utils"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService services.ChatService
	streams     map[string]context.CancelFunc // Track active streams
	streamsMux  sync.RWMutex                  // Mutex for thread-safe streams map access
}

func NewChatHandler(chatService services.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		streams:     make(map[string]context.CancelFunc),
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

	response, statusCode, err := h.chatService.CreateMessage(userID, chatID, &req)
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

// StreamResponse handles SSE streaming of AI responses
func (h *ChatHandler) StreamResponse(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("streamId")

	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Store cancel function for potential cancellation
	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)
	h.streamsMux.Lock()
	h.streams[streamKey] = cancel
	h.streamsMux.Unlock()
	defer func() {
		h.streamsMux.Lock()
		delete(h.streams, streamKey)
		h.streamsMux.Unlock()
	}()

	// Get stream channel from service
	streamChan, err := h.chatService.StreamResponse(userID, chatID, streamID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusInternalServerError, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			// Send completion event
			data := dtos.StreamResponse{
				Event: "complete",
				Data:  nil,
			}
			json.NewEncoder(w).Encode(data)
			return false

		case response, ok := <-streamChan:
			if !ok {
				return false
			}
			// Send chunk event
			json.NewEncoder(w).Encode(response)
			return true

		case <-time.After(30 * time.Second):
			// Send keepalive event
			data := dtos.StreamResponse{
				Event: "keepalive",
				Data:  nil,
			}
			json.NewEncoder(w).Encode(data)
			return true
		}
	})
}

// CancelStream cancels an ongoing stream
func (h *ChatHandler) CancelStream(c *gin.Context) {
	userID := c.GetString("userID")
	chatID := c.Param("id")
	streamID := c.Query("streamId")

	streamKey := fmt.Sprintf("%s:%s:%s", userID, chatID, streamID)

	h.streamsMux.RLock()
	cancel, exists := h.streams[streamKey]
	h.streamsMux.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, dtos.Response{
			Success: false,
			Error:   utils.ToStringPtr("No active stream found"),
		})
		return
	}

	// Cancel the stream
	cancel()

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    "Stream cancelled successfully",
	})
}
