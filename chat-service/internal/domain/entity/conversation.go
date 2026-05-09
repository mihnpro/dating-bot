package entity

import "time"

type ConversationStatus string

const (
	ConversationStatusActive  ConversationStatus = "active"
	ConversationStatusBlocked ConversationStatus = "blocked"
)

type Conversation struct {
	ID        string
	MatchID   int64
	User1ID   int64
	User2ID   int64
	Status    ConversationStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewConversation(matchID, user1ID, user2ID int64) *Conversation {
	now := time.Now()
	return &Conversation{
		MatchID:   matchID,
		User1ID:   user1ID,
		User2ID:   user2ID,
		Status:    ConversationStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (c *Conversation) HasParticipant(userID int64) bool {
	return c.User1ID == userID || c.User2ID == userID
}

func (c *Conversation) OtherParticipant(userID int64) int64 {
	if c.User1ID == userID {
		return c.User2ID
	}
	return c.User1ID
}
