package routes

import "github.com/gin-gonic/gin"

func SetupDefaultRoutes(router *gin.Engine) {
	// Health check route
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello, World!",
		})
	})
}
