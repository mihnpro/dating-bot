package entity

import (
	"encoding/json"
	"time"
)

// DomainEvent is the envelope for every event that travels over RabbitMQ.
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

func UnmarshalDomainEvent(raw []byte) (*DomainEvent, error) {
	var e DomainEvent
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// ── Inbound event payloads that the Recommendation Service cares about ──────

// ProfileUpdatedData is published by user-profile-service on profile.updated.
type ProfileUpdatedData struct {
	UserID int64 `json:"user_id"`
}

// InteractionLikedData is published by matching-service on interaction.liked.
type InteractionLikedData struct {
	FromUserID int64 `json:"from_user_id"`
	ToUserID   int64 `json:"to_user_id"`
}

// ── Outbound event payloads ──────────────────────────────────────────────────

// RatingRecalculatedData is published by the Recommendation Service after a
// successful recalculation so that other services can react if needed.
type RatingRecalculatedData struct {
	UserID           int64   `json:"user_id"`
	PrimaryRating    float64 `json:"primary_rating"`
	BehavioralRating float64 `json:"behavioral_rating"`
	CombinedRating   float64 `json:"combined_rating"`
}

func NewRatingRecalculatedEvent(
	userID int64,
	primary, behavioral, combined float64,
) (*DomainEvent, error) {
	return NewDomainEvent("rating.recalculated", RatingRecalculatedData{
		UserID:           userID,
		PrimaryRating:    primary,
		BehavioralRating: behavioral,
		CombinedRating:   combined,
	})
}
