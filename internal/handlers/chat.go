package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jeff-kidzie/floops-be/internal/chat"
	"github.com/jeff-kidzie/floops-be/internal/database"
	"github.com/jeff-kidzie/floops-be/internal/middleware"
	"github.com/jeff-kidzie/floops-be/internal/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type incomingMessage struct {
	Content string `json:"content"`
}

func HandleWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token required"})
		return
	}

	claims, err := middleware.ParseTokenString(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	sub, _ := claims["sub"].(float64)
	userID := int(sub)
	username, _ := claims["username"].(string)

	conversationIDStr := c.Param("conversationID")
	conversationID, err := strconv.Atoi(conversationIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation ID"})
		return
	}

	// Verify membership
	var exists bool
	err = database.DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id = $1 AND user_id = $2)`,
		conversationID, userID,
	).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this conversation"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	client := &chat.Client{
		UserID:         userID,
		Username:       username,
		ConversationID: conversationID,
		Conn:           conn,
		Send:           make(chan []byte, 256),
		OnMessage:      handleIncomingMessage,
	}

	chat.GlobalHub.Register(client)
	go client.WritePump()
	go client.ReadPump()
}

func handleIncomingMessage(client *chat.Client, data []byte) {
	var msg incomingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	if msg.Content == "" {
		return
	}

	var dbMsg models.Message
	err := database.DB.QueryRow(
		`INSERT INTO messages (conversation_id, sender_id, content, created_at) VALUES ($1, $2, $3, $4) RETURNING id, conversation_id, sender_id, content, created_at`,
		client.ConversationID, client.UserID, msg.Content, time.Now(),
	).Scan(&dbMsg.ID, &dbMsg.ConversationID, &dbMsg.SenderID, &dbMsg.Content, &dbMsg.CreatedAt)
	if err != nil {
		log.Printf("insert message error: %v", err)
		return
	}

	dbMsg.SenderUsername = client.Username

	broadcast, err := json.Marshal(dbMsg)
	if err != nil {
		log.Printf("marshal message error: %v", err)
		return
	}

	chat.GlobalHub.Broadcast(client.ConversationID, broadcast)
}

type createConversationRequest struct {
	UserID int `json:"user_id" binding:"required"`
}

func CreateConversation(c *gin.Context) {
	currentUserID := c.GetInt("userID")

	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create conversation with yourself"})
		return
	}

	// Check if target user exists
	var targetExists bool
	if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, req.UserID).Scan(&targetExists); err != nil || !targetExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if conversation already exists between these two users
	var existingID int
	err := database.DB.QueryRow(`
		SELECT cm1.conversation_id FROM conversation_members cm1
		JOIN conversation_members cm2 ON cm1.conversation_id = cm2.conversation_id
		WHERE cm1.user_id = $1 AND cm2.user_id = $2
	`, currentUserID, req.UserID).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"conversation_id": existingID})
		return
	}

	// Create conversation in a transaction
	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversation"})
		return
	}
	defer tx.Rollback()

	var convoID int
	if err := tx.QueryRow(`INSERT INTO conversations (created_at) VALUES ($1) RETURNING id`, time.Now()).Scan(&convoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversation"})
		return
	}

	_, err = tx.Exec(`INSERT INTO conversation_members (conversation_id, user_id, joined_at) VALUES ($1, $2, $3), ($1, $4, $3)`,
		convoID, currentUserID, time.Now(), req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add members"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"conversation_id": convoID})
}

func GetConversations(c *gin.Context) {
	userID := c.GetInt("userID")

	rows, err := database.DB.Query(`
		SELECT c.id, c.created_at, u.id, u.username
		FROM conversations c
		JOIN conversation_members cm1 ON c.id = cm1.conversation_id AND cm1.user_id = $1
		JOIN conversation_members cm2 ON c.id = cm2.conversation_id AND cm2.user_id != $1
		JOIN users u ON cm2.user_id = u.id
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch conversations"})
		return
	}
	defer rows.Close()

	type conversationResponse struct {
		ID            int       `json:"id"`
		CreatedAt     time.Time `json:"created_at"`
		OtherUserID   int       `json:"other_user_id"`
		OtherUsername string    `json:"other_username"`
	}

	conversations := []conversationResponse{}
	for rows.Next() {
		var cr conversationResponse
		if err := rows.Scan(&cr.ID, &cr.CreatedAt, &cr.OtherUserID, &cr.OtherUsername); err != nil {
			continue
		}
		conversations = append(conversations, cr)
	}

	c.JSON(http.StatusOK, conversations)
}

func GetMessages(c *gin.Context) {
	userID := c.GetInt("userID")
	conversationID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation ID"})
		return
	}

	// Verify membership
	var isMember bool
	if err := database.DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id = $1 AND user_id = $2)`,
		conversationID, userID,
	).Scan(&isMember); err != nil || !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this conversation"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	query := `
		SELECT m.id, m.conversation_id, m.sender_id, u.username, m.content, m.created_at
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.conversation_id = $1
	`
	args := []any{conversationID}

	if before := c.Query("before"); before != "" {
		if beforeID, err := strconv.Atoi(before); err == nil {
			query += ` AND m.id < $2 ORDER BY m.id DESC LIMIT $3`
			args = append(args, beforeID, limit)
		}
	} else {
		query += ` ORDER BY m.id DESC LIMIT $2`
		args = append(args, limit)
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}
	defer rows.Close()

	messages := []models.Message{}
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderUsername, &m.Content, &m.CreatedAt); err != nil {
			continue
		}
		messages = append(messages, m)
	}

	c.JSON(http.StatusOK, messages)
}
