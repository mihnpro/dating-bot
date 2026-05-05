package event

import (
	"encoding/json"
	"time"
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

type MediaUploadedData struct {
	UserID  int64 `json:"user_id"`
	MediaID int64 `json:"media_id"`
}

func NewMediaUploadedEvent(userID, mediaID int64) (*DomainEvent, error) {
	return NewDomainEvent("media.uploaded", MediaUploadedData{
		UserID:  userID,
		MediaID: mediaID,
	})
}
