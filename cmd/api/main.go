package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jeff-kidzie/floops-be/internal/database"
	"github.com/jeff-kidzie/floops-be/internal/handlers"
	"github.com/joho/godotenv"
)

func main() {
	loadDb()
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "Floops backend is running",
			"port":   "8080",
		})
	})

	router.GET("/db-health", func(c *gin.Context) {
		if err := database.DB.Ping(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "Database connection failed",
				"error":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "Database connection is healthy",
		})
	})

	auth := router.Group("/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
		auth.POST("/forgot-password", handlers.ForgotPassword)
	}

	router.Run(":8080")
}

func loadDb() {
	// Load environment variables from .env file
	err := godotenv.Load("local.env")
	if err != nil {
		log.Fatalf("Error loading local.env file: %v", err)
	}

	if err := database.Connect(); err != nil {
		log.Fatalf("could not connect to DB: %v", err)
	}
}
