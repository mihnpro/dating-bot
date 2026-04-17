package event

import (
	"github.com/dating-bot/recommendation-service/internal/domain/entity"
)

// Re-export entity event types so callers can use either package path.

type DomainEvent = entity.DomainEvent

type ProfileUpdatedData = entity.ProfileUpdatedData
type InteractionLikedData = entity.InteractionLikedData
type RatingRecalculatedData = entity.RatingRecalculatedData

var (
	NewDomainEvent             = entity.NewDomainEvent
	UnmarshalDomainEvent       = entity.UnmarshalDomainEvent
	NewRatingRecalculatedEvent = entity.NewRatingRecalculatedEvent
)
