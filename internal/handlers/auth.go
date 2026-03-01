package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jeff-kidzie/floops-be/internal/database"
	"github.com/jeff-kidzie/floops-be/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := strings.TrimSpace(req.Email)
	username := strings.TrimSpace(req.Username)
	password := req.Password

	if email == "" || username == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email, username, and password are required"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	query := `INSERT INTO users (email, username, password_hash, created_at) VALUES ($1, $2, $3, $4) RETURNING id,email,username,created_at`
	var user models.User
	err = database.DB.QueryRow(query, email, username, string(hash), time.Now()).Scan(&user.ID, &user.Email, &user.Username, &user.CreatedAt)
	if err != nil {
		log.Printf("insert user error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, user)
}

func Login(c *gin.Context) {
	// var req LoginRequest
	// if err := c.ShouldBindJSON(&req); err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	// 	return
	// }
}

func ForgotPassword(c *gin.Context) {}
