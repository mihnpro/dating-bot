package event

import (
	"encoding/json"
	"time"

	"github.com/dating-bot/matching-service/internal/domain/entity"
)

type DomainEvent struct {
	EventName string          `json:"event_name"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

func NewDomainEvent(eventName string, data any) (*DomainEvent, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &DomainEvent{
		EventName: eventName,
		Timestamp: time.Now(),
		Data:      payload,
	}, nil
}

func (e *DomainEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func UnmarshalDomainEvent(data []byte) (*DomainEvent, error) {
	var e DomainEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// --- Event payloads ---

type MatchCreatedData struct {
	MatchID  int64 `json:"match_id"`
	User1ID  int64 `json:"user1_id"`
	User2ID  int64 `json:"user2_id"`
}

type InteractionLikedData struct {
	FromUserID int64 `json:"from_user_id"`
	ToUserID   int64 `json:"to_user_id"`
}

// --- Factory helpers ---

func NewMatchCreatedEvent(match *entity.Match) (*DomainEvent, error) {
	return NewDomainEvent("match.created", MatchCreatedData{
		MatchID: match.ID,
		User1ID: match.User1ID,
		User2ID: match.User2ID,
	})
}

func NewInteractionLikedEvent(fromUserID, toUserID int64) (*DomainEvent, error) {
	return NewDomainEvent("interaction.liked", InteractionLikedData{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
	})
}
