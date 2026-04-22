package main

import (
	"log"
	"os"

	"security-service/internal/middleware"
	"security-service/internal/store"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize database
	db, err := store.InitDB()
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Initialize Redis
	rdb, err := store.InitRedis()
	if err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}

	_ = db
	_ = rdb

	r := gin.Default()

	// Global middleware
	r.Use(middleware.Cors())
	r.Use(middleware.RequestID())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// TODO: Register route groups here
	// api := r.Group("/api/v1")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
