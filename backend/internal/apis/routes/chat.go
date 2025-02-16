package routes

import (
	"log"
	"neobase-ai/internal/apis/middlewares"
	"neobase-ai/internal/di"

	"github.com/gin-gonic/gin"
)

func SetupChatRoutes(router *gin.Engine) {
	chatHandler, err := di.GetChatHandler()
	if err != nil {
		log.Fatalf("Failed to get chat handler: %v", err)
	}

	protected := router.Group("/api/chats")
	protected.Use(middlewares.AuthMiddleware())
	{
		// Chat CRUD
		protected.POST("", chatHandler.Create)
		protected.GET("", chatHandler.List)
		protected.GET("/:id", chatHandler.GetByID)
		protected.PUT("/:id", chatHandler.Update)
		protected.DELETE("/:id", chatHandler.Delete)

		// Messages within a chat
		protected.GET("/:id/messages", chatHandler.ListMessages)
		protected.POST("/:id/messages", chatHandler.CreateMessage)
		protected.PATCH("/:id/messages/:messageId", chatHandler.UpdateMessage)
		protected.DELETE("/:id/messages", chatHandler.DeleteMessages)

		// SSE endpoints for streaming
		protected.GET("/:id/stream", chatHandler.StreamResponse)       // Listen to AI response stream
		protected.POST("/:id/stream/cancel", chatHandler.CancelStream) // Cancel ongoing stream
	}
}
