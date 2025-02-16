package routes

import (
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupDefaultRoutes(router *gin.Engine) {
	// Add recovery middleware
	router.Use(middleware.CustomRecoveryMiddleware())

	// Health check route
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, dtos.Response{
			Success: true,
			Data:    "Server is healthy!",
		})
	})

	// Setup all route groups
	SetupAuthRoutes(router)
	SetupChatRoutes(router)
}
