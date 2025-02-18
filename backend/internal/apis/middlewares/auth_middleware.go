package middlewares

import (
	"log"
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/di"
	"neobase-ai/internal/repositories"
	"neobase-ai/internal/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var jwtService *utils.JWTService
var tokenRepo repositories.TokenRepository

func AuthMiddleware() gin.HandlerFunc {
	if jwtService == nil {
		if err := di.DiContainer.Invoke(func(service utils.JWTService) {
			jwtService = &service
		}); err != nil {
			log.Fatalf("Failed to provide JWT service: %v", err)
		}
	}
	if tokenRepo == nil {
		if err := di.DiContainer.Invoke(func(repo repositories.TokenRepository) {
			tokenRepo = repo
		}); err != nil {
			log.Fatalf("Failed to provide Token repository: %v", err)
		}
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			errorMsg := "Authorization header is required"
			c.JSON(http.StatusUnauthorized, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			errorMsg := "Invalid authorization header format"
			c.JSON(http.StatusUnauthorized, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			c.Abort()
			return
		}

		token := parts[1]

		// Check if token is blacklisted
		if tokenRepo.IsTokenBlacklisted(token) {
			errorMsg := "Token has been revoked"
			c.JSON(http.StatusUnauthorized, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			c.Abort()
			return
		}

		claims, err := (*jwtService).ValidateToken(token)
		if err != nil {
			errorMsg := "Invalid or expired token"
			c.JSON(http.StatusUnauthorized, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			c.Abort()
			return
		}
		log.Printf("User ID from Auth Middleware: %s", *claims)

		c.Set("userID", *claims)
		c.Next()
	}
}
