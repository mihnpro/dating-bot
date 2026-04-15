package entity

import "time"

type MatchStatus string

const (
	MatchStatusActive    MatchStatus = "active"
	MatchStatusExpired   MatchStatus = "expired"
	MatchStatusBlocked   MatchStatus = "blocked"
)

type Match struct {
	ID                 int64
	User1ID            int64
	User2ID            int64
	CreatedAt          time.Time
	Status             MatchStatus
	ConversationStarted bool
	LastInteractionAt  time.Time
}

func NewMatch(user1ID, user2ID int64) *Match {
	now := time.Now()
	return &Match{
		User1ID:            user1ID,
		User2ID:            user2ID,
		CreatedAt:          now,
		Status:             MatchStatusActive,
		ConversationStarted: false,
		LastInteractionAt:  now,
	}
}

func (m *Match) StartConversation() {
	m.ConversationStarted = true
	m.LastInteractionAt = time.Now()
}
