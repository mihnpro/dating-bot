package entity

import "time"

type NotificationType string

const (
	TypeMatchCreated  NotificationType = "match_created"
	TypeNewLike       NotificationType = "new_like"
	TypeNewMessage    NotificationType = "new_message"
)

type Notification struct {
	ID        int64
	UserID    int64
	Type      NotificationType
	Message   string
	IsSent    bool
	IsRead    bool
	CreatedAt time.Time
	SentAt    *time.Time
}

func NewNotification(userID int64, notifType NotificationType, message string) *Notification {
	return &Notification{
		UserID:    userID,
		Type:      notifType,
		Message:   message,
		IsSent:    false,
		IsRead:    false,
		CreatedAt: time.Now(),
	}
}
