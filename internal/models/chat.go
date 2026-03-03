package models

import "time"

type Conversation struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

type ConversationMember struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	UserID         int       `json:"user_id"`
	JoinedAt       time.Time `json:"joined_at"`
}

type Message struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	SenderID       int       `json:"sender_id"`
	SenderUsername string    `json:"sender_username,omitempty"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}
