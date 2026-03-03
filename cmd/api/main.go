package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jeff-kidzie/floops-be/internal/chat"
	"github.com/jeff-kidzie/floops-be/internal/database"
	"github.com/jeff-kidzie/floops-be/internal/handlers"
	"github.com/jeff-kidzie/floops-be/internal/middleware"
	"github.com/joho/godotenv"
)

func main() {
	loadDb()

	chat.GlobalHub = chat.NewHub()
	go chat.GlobalHub.Run()

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

	handlers.InitGoogleOAuth()

	auth := router.Group("/auth")
	{
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
		auth.POST("/forgot-password", handlers.ForgotPassword)
		auth.GET("/google/login", handlers.GoogleLogin)
		auth.GET("/google/callback", handlers.GoogleCallback)
	}

	api := router.Group("/api", middleware.AuthRequired())
	{
		api.POST("/conversations", handlers.CreateConversation)
		api.GET("/conversations", handlers.GetConversations)
		api.GET("/conversations/:id/messages", handlers.GetMessages)
	}

	router.GET("/ws/:conversationID", handlers.HandleWebSocket)

	router.Run(":8080")
}

func loadDb() {
	// Load environment variables from local.env if present (ignored in containerized environments)
	if err := godotenv.Load("local.env"); err != nil {
		log.Println("local.env not found, using environment variables")
	}

	if err := database.Connect(); err != nil {
		log.Fatalf("could not connect to DB: %v", err)
	}
}
