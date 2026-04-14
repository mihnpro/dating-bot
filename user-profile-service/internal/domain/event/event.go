package event

import (
	"encoding/json"
	"time"

	"github.com/dating-bot/user-profile-service/internal/domain/entity"
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

type UserRegisteredData struct {
	UserID     int64  `json:"user_id"`
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username"`
}

type UserUpdatedData struct {
	UserID int64 `json:"user_id"`
}

type ProfileUpdatedData struct {
	UserID int64 `json:"user_id"`
}

// --- Factory helpers ---

func NewUserRegisteredEvent(user *entity.User) (*DomainEvent, error) {
	return NewDomainEvent("user.registered", UserRegisteredData{
		UserID:     user.ID,
		TelegramID: user.TelegramID,
		Username:   user.Username,
	})
}

func NewUserUpdatedEvent(user *entity.User) (*DomainEvent, error) {
	return NewDomainEvent("user.updated", UserUpdatedData{
		UserID: user.ID,
	})
}

func NewProfileUpdatedEvent(profile *entity.Profile) (*DomainEvent, error) {
	return NewDomainEvent("profile.updated", ProfileUpdatedData{
		UserID: profile.UserID,
	})
}
