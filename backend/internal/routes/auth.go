package routes

import (
	"neobase-ai/internal/di"
	"neobase-ai/internal/handlers"

	"github.com/gin-gonic/gin"
)

func SetupAuthRoutes(router *gin.Engine) {
	var authHandler handlers.AuthHandler
	di.DiContainer.Invoke(func(handler *handlers.AuthHandler) {
		authHandler = *handler
	})

	// Auth routes
	auth := router.Group("/api/auth")
	{
		auth.POST("/signup", authHandler.Signup)
		auth.POST("/login", authHandler.Login)
	}

}
