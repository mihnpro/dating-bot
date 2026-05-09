package event

import (
	"encoding/json"
	"fmt"
	"time"
)

type DomainEvent struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

func New(eventType string, payload any) (*DomainEvent, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return &DomainEvent{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:       eventType,
		OccurredAt: time.Now(),
		Payload:    data,
	}, nil
}

func (e *DomainEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// MatchCreatedPayload is published by matching-service on match.created
type MatchCreatedPayload struct {
	MatchID int64 `json:"match_id"`
	User1ID int64 `json:"user1_id"`
	User2ID int64 `json:"user2_id"`
}

// MessageSentPayload is published by chat-service on chat.message_sent
type MessageSentPayload struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	SenderID       int64  `json:"sender_id"`
}
