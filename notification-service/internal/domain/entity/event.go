package entity

import (
	"encoding/json"
	"time"
)

// RawEvent handles both event formats published in the system:
//   - matching-service: {"event_name":"...", "timestamp":"...", "data":{...}}
//   - chat-service:     {"type":"...", "occurred_at":"...", "payload":{...}}
type RawEvent struct {
	// matching-service / recommendation-service / user-profile-service format
	EventName string          `json:"event_name"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`

	// chat-service format
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

func UnmarshalRawEvent(body []byte) (*RawEvent, error) {
	var e RawEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// Name returns the event name regardless of which format was used.
func (e *RawEvent) Name() string {
	if e.EventName != "" {
		return e.EventName
	}
	return e.Type
}

// RawPayload returns the event payload regardless of which format was used.
func (e *RawEvent) RawPayload() json.RawMessage {
	if len(e.Data) > 0 {
		return e.Data
	}
	return e.Payload
}

// ── Inbound event payloads ────────────────────────────────────────────────────

// MatchCreatedData is published by matching-service on match.created.
type MatchCreatedData struct {
	MatchID int64 `json:"match_id"`
	User1ID int64 `json:"user1_id"`
	User2ID int64 `json:"user2_id"`
}

// InteractionLikedData is published by matching-service on interaction.liked.
type InteractionLikedData struct {
	FromUserID int64 `json:"from_user_id"`
	ToUserID   int64 `json:"to_user_id"`
}

// MessageSentData is published by chat-service on chat.message_sent.
type MessageSentData struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	SenderID       int64  `json:"sender_id"`
}
