package middlewares

import (
	"neobase-ai/internal/di"
	"neobase-ai/internal/dtos"
	"neobase-ai/internal/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	var jwtService utils.JWTService
	di.DiContainer.Invoke(func(service utils.JWTService) {
		jwtService = service
	})

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
			errorMsg := "Invalid authorization format. Use: Bearer <token>"
			c.JSON(http.StatusUnauthorized, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			c.Abort()
			return
		}

		userID, err := jwtService.ValidateToken(parts[1])
		if err != nil {
			errorMsg := "Invalid or expired token"
			c.JSON(http.StatusUnauthorized, dtos.Response{
				Success: false,
				Error:   &errorMsg,
			})
			c.Abort()
			return
		}

		// Set userID in context for later use
		c.Set("userID", userID)
		c.Next()
	}
}
