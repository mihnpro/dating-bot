package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/dating-bot/notification-service/internal/client"
	"github.com/dating-bot/notification-service/internal/domain/entity"
	"github.com/dating-bot/notification-service/internal/domain/repository"
	"github.com/dating-bot/notification-service/internal/transport/metrics"
)

const (
	msgMatch   = "🎉 У тебя новый мэтч! Открой /matches, чтобы начать общаться."
	msgLike    = "❤️ Кто-то поставил тебе лайк! Зайди в /browse, чтобы ответить."
	msgMessage = "💬 У тебя новое сообщение! Открой /matches, чтобы ответить."
)

type NotificationService struct {
	repo      repository.NotificationRepository
	upClient  *client.UserProfileClient
	chatClient *client.ChatClient
	gwClient  *client.GatewayClient
}

func NewNotificationService(
	repo repository.NotificationRepository,
	upClient *client.UserProfileClient,
	chatClient *client.ChatClient,
	gwClient *client.GatewayClient,
) *NotificationService {
	return &NotificationService{
		repo:       repo,
		upClient:   upClient,
		chatClient: chatClient,
		gwClient:   gwClient,
	}
}

// HandleEvent is the single entry point for all raw RabbitMQ messages.
// It decodes the envelope and routes to the appropriate handler.
func (s *NotificationService) HandleEvent(body []byte) error {
	evt, err := entity.UnmarshalRawEvent(body)
	if err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	metrics.IncEventsConsumed(evt.Name())

	var handlerErr error
	switch evt.Name() {
	case "match.created":
		handlerErr = s.handleMatchCreated(evt)
	case "interaction.liked":
		handlerErr = s.handleInteractionLiked(evt)
	case "chat.message_sent":
		handlerErr = s.handleMessageSent(evt)
	default:
		return nil
	}
	if handlerErr != nil {
		metrics.IncEventsError(evt.Name())
	}
	return handlerErr
}

// ── Event handlers ────────────────────────────────────────────────────────────

func (s *NotificationService) handleMatchCreated(evt *entity.RawEvent) error {
	var data entity.MatchCreatedData
	if err := json.Unmarshal(evt.RawPayload(), &data); err != nil {
		return fmt.Errorf("unmarshal match.created payload: %w", err)
	}

	ctx := context.Background()

	// Notify both participants concurrently.
	for _, userID := range []int64{data.User1ID, data.User2ID} {
		if err := s.deliver(ctx, userID, entity.TypeMatchCreated, msgMatch); err != nil {
			logrus.WithError(err).WithField("user_id", userID).
				Warn("failed to deliver match notification")
		}
	}
	return nil
}

func (s *NotificationService) handleInteractionLiked(evt *entity.RawEvent) error {
	var data entity.InteractionLikedData
	if err := json.Unmarshal(evt.RawPayload(), &data); err != nil {
		return fmt.Errorf("unmarshal interaction.liked payload: %w", err)
	}

	return s.deliver(context.Background(), data.ToUserID, entity.TypeNewLike, msgLike)
}

func (s *NotificationService) handleMessageSent(evt *entity.RawEvent) error {
	var data entity.MessageSentData
	if err := json.Unmarshal(evt.RawPayload(), &data); err != nil {
		return fmt.Errorf("unmarshal chat.message_sent payload: %w", err)
	}

	ctx := context.Background()

	// Resolve the receiver: chat-service only gives us sender + conversation.
	conv, err := s.chatClient.GetConversation(ctx, data.ConversationID)
	if err != nil {
		return fmt.Errorf("resolve conversation %s: %w", data.ConversationID, err)
	}

	receiverID := conv.User1ID
	if conv.User1ID == data.SenderID {
		receiverID = conv.User2ID
	}

	return s.deliver(ctx, receiverID, entity.TypeNewMessage, msgMessage)
}

// ── Delivery ──────────────────────────────────────────────────────────────────

// deliver saves a notification to DB and pushes it to the user via Telegram.
func (s *NotificationService) deliver(
	ctx context.Context,
	userID int64,
	notifType entity.NotificationType,
	message string,
) error {
	n := entity.NewNotification(userID, notifType, message)
	if err := s.repo.Create(ctx, n); err != nil {
		return fmt.Errorf("save notification: %w", err)
	}
	metrics.IncCreated(string(notifType))

	telegramID, err := s.upClient.GetTelegramID(ctx, userID)
	if err != nil {
		logrus.WithError(err).WithField("user_id", userID).
			Warn("could not resolve telegram_id — notification saved but not delivered")
		return nil
	}
	if telegramID == 0 {
		logrus.WithField("user_id", userID).Warn("user not found in user-profile-service")
		return nil
	}

	if err := s.gwClient.SendMessage(ctx, telegramID, message); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"user_id":     userID,
			"telegram_id": telegramID,
		}).Warn("gateway delivery failed — notification saved, delivery pending")
		return nil
	}

	if err := s.repo.MarkSent(ctx, n.ID); err != nil {
		logrus.WithError(err).WithField("notification_id", n.ID).
			Warn("mark notification as sent failed")
	}

	logrus.WithFields(logrus.Fields{
		"user_id":     userID,
		"telegram_id": telegramID,
		"type":        notifType,
	}).Info("notification delivered")

	return nil
}

// ── Read-side queries (used by HTTP handler) ──────────────────────────────────

func (s *NotificationService) GetNotifications(
	ctx context.Context, userID int64, limit, offset int,
) ([]*entity.Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.GetByUserID(ctx, userID, limit, offset)
}

func (s *NotificationService) MarkRead(ctx context.Context, id int64) error {
	return s.repo.MarkRead(ctx, id)
}

func (s *NotificationService) CountUnread(ctx context.Context, userID int64) (int, error) {
	return s.repo.CountUnread(ctx, userID)
}
