package middleware

import (
	"fmt"
	"neobase-ai/internal/apis/dtos"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// CustomRecoveryMiddleware handles panics and returns a proper response DTO
func CustomRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Print stack trace
				debugStack := string(debug.Stack())
				fmt.Printf("Recovery from panic: %v\nStack Trace:\n%s\n", err, debugStack)

				// Create error message
				errorMsg := "Internal Server Error"
				if gin.IsDebugging() {
					errorMsg = fmt.Sprintf("Internal Server Error: %v", err)
				}

				// Return error response using response DTO
				c.AbortWithStatusJSON(500, dtos.Response{
					Success: false,
					Error:   &errorMsg,
					Data:    nil,
				})
			}
		}()
		c.Next()
	}
}
