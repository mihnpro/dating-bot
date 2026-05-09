package entity

import "time"

type ContentType string

const ContentTypeText ContentType = "text"

type Message struct {
	ID             string
	ConversationID string
	SenderID       int64
	Content        string
	ContentType    ContentType
	SentAt         time.Time
	IsRead         bool
}

func NewMessage(conversationID string, senderID int64, content string) *Message {
	return &Message{
		ConversationID: conversationID,
		SenderID:       senderID,
		Content:        content,
		ContentType:    ContentTypeText,
		SentAt:         time.Now(),
		IsRead:         false,
	}
}
