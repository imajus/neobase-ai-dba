package main

import (
	"context"
	"log"
	"neobase-ai/config"
	"neobase-ai/internal/di"
	"neobase-ai/internal/routes"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load environment variables
	err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Failed to load environment variables: %v", err)
	}

	// Initialize dependencies
	di.Initialize()

	// Setup Gin
	ginApp := gin.Default()
	ginApp.Use(cors.Default())

	// Setup routes
	routes.SetupDefaultRoutes(ginApp)

	// Create server
	srv := &http.Server{
		Addr:    ":" + config.Env.Port,
		Handler: ginApp,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", config.Env.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
