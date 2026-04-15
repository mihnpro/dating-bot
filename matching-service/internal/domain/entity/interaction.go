package entity

import "time"

type InteractionType string

const (
	InteractionTypeLike  InteractionType = "like"
	InteractionTypePass  InteractionType = "pass"
)

type Interaction struct {
	ID         int64
	FromUserID int64
	ToUserID   int64
	Type       InteractionType
	CreatedAt  time.Time
	TimeOfDay  int // hour 0-23
}

func NewInteraction(fromUserID, toUserID int64, t InteractionType) *Interaction {
	now := time.Now()
	return &Interaction{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Type:       t,
		CreatedAt:  now,
		TimeOfDay:  now.Hour(),
	}
}
