package routes

import (
	"neobase-ai/internal/apis/handlers"
	"neobase-ai/internal/apis/middlewares"

	"github.com/gin-gonic/gin"
)

// SetupVisualizationRoutes configures routes for visualization APIs
func SetupVisualizationRoutes(router *gin.Engine, visualizationHandler *handlers.VisualizationHandler) {
	// Visualization API routes - all protected by authentication
	visualizationGroup := router.Group("/api/visualizations")
	visualizationGroup.Use(middlewares.AuthMiddleware())

	// Get visualization suggestions for a specific table
	visualizationGroup.GET("/suggestions/table", visualizationHandler.GetTableSuggestions)

	// Get visualization suggestions for all tables
	visualizationGroup.GET("/suggestions", visualizationHandler.GetAllSuggestions)

	// Execute visualization query
	visualizationGroup.POST("/execute", visualizationHandler.ExecuteVisualization)
}
