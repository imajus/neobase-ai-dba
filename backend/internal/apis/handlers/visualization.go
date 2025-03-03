package handlers

import (
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// VisualizationHandler handles APIs related to visualizations
type VisualizationHandler struct {
	visualizationService services.VisualizationService
}

// NewVisualizationHandler creates a new visualization handler
func NewVisualizationHandler(visualizationService services.VisualizationService) *VisualizationHandler {
	return &VisualizationHandler{
		visualizationService: visualizationService,
	}
}

// GetTableSuggestions returns visualization suggestions for a specific table
func (h *VisualizationHandler) GetTableSuggestions(c *gin.Context) {
	chatID := c.Query("chat_id")
	tableName := c.Query("table_name")

	if chatID == "" {
		errorMsg := "chat_id is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	if tableName == "" {
		errorMsg := "table_name is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Get suggestions
	suggestions, err := h.visualizationService.GetTableSuggestions(c.Request.Context(), chatID, tableName)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusInternalServerError, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    suggestions,
	})
}

// GetAllSuggestions returns visualization suggestions for all tables
func (h *VisualizationHandler) GetAllSuggestions(c *gin.Context) {
	chatID := c.Query("chat_id")

	if chatID == "" {
		errorMsg := "chat_id is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Get suggestions for all tables
	suggestions, err := h.visualizationService.GetAllSuggestions(c.Request.Context(), chatID)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusInternalServerError, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    suggestions,
	})
}

// ExecuteVisualization executes a visualization query and returns the data
func (h *VisualizationHandler) ExecuteVisualization(c *gin.Context) {
	chatID := c.Query("chat_id")
	streamID := c.Query("stream_id")

	if chatID == "" {
		errorMsg := "chat_id is required"
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Stream ID can be optional
	if streamID == "" {
		streamID = "visualization-" + chatID
	}

	// Parse the request body
	var suggestion services.VisualizationSuggestion
	if err := c.ShouldBindJSON(&suggestion); err != nil {
		errorMsg := "Invalid request body: " + err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	// Execute the visualization
	data, err := h.visualizationService.ExecuteVisualization(c.Request.Context(), chatID, streamID, suggestion)
	if err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusInternalServerError, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    data,
	})
}
