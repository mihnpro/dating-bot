package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dating-bot/chat-service/internal/domain/entity"
	"github.com/dating-bot/chat-service/internal/domain/event"
	"github.com/dating-bot/chat-service/internal/domain/repository"
	"github.com/sirupsen/logrus"
)

// Hub is the minimal interface the service needs to broadcast messages in real-time.
type Hub interface {
	Broadcast(recipientIDs []int64, payload []byte)
}

// UserProfileFetcher retrieves basic user info from user-profile-service.
// Defined here so the service layer stays independent of the gRPC client package.
type UserProfileFetcher interface {
	GetUser(ctx context.Context, userID int64) (*UserInfo, error)
	GetUsersBatch(ctx context.Context, userIDs []int64) (map[int64]*UserInfo, error)
}

// UserInfo is the subset of user-profile data the chat service needs.
// Duplicated from client package to avoid a cross-layer import.
type UserInfo struct {
	UserID    int64
	FirstName string
	Username  string
	Age       int32
	City      string
	Gender    string
}

type ChatService struct {
	convRepo  repository.ConversationRepository
	msgRepo   repository.MessageRepository
	publisher repository.EventPublisher
	hub       Hub
	upClient  UserProfileFetcher // optional — nil if user-profile-service is unreachable
}

func NewChatService(
	convRepo repository.ConversationRepository,
	msgRepo repository.MessageRepository,
	publisher repository.EventPublisher,
	hub Hub,
	upClient UserProfileFetcher,
) *ChatService {
	return &ChatService{
		convRepo:  convRepo,
		msgRepo:   msgRepo,
		publisher: publisher,
		hub:       hub,
		upClient:  upClient,
	}
}

// GetOrCreateConversation is idempotent — returns existing conversation for the match.
func (s *ChatService) GetOrCreateConversation(ctx context.Context, matchID, user1ID, user2ID int64) (*entity.Conversation, error) {
	existing, err := s.convRepo.GetByMatchID(ctx, matchID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("lookup conversation: %w", err)
	}
	if existing != nil {
		return existing, nil
	}
	conv := entity.NewConversation(matchID, user1ID, user2ID)
	if err := s.convRepo.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return conv, nil
}

func (s *ChatService) GetConversation(ctx context.Context, id string) (*entity.Conversation, error) {
	return s.convRepo.GetByID(ctx, id)
}

func (s *ChatService) GetUserConversations(ctx context.Context, userID int64) ([]*entity.Conversation, error) {
	return s.convRepo.GetByUserID(ctx, userID)
}

// GetUserConversationsEnriched returns conversations enriched with the other
// participant's display name fetched via gRPC from user-profile-service.
func (s *ChatService) GetUserConversationsEnriched(ctx context.Context, userID int64) ([]*EnrichedConversation, error) {
	convs, err := s.convRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]*EnrichedConversation, 0, len(convs))

	if s.upClient == nil || len(convs) == 0 {
		for _, c := range convs {
			result = append(result, &EnrichedConversation{Conversation: c})
		}
		return result, nil
	}

	// Collect other-participant IDs for a single batch gRPC call.
	otherIDs := make([]int64, 0, len(convs))
	for _, c := range convs {
		otherIDs = append(otherIDs, c.OtherParticipant(userID))
	}

	profiles, err := s.upClient.GetUsersBatch(ctx, otherIDs)
	if err != nil {
		logrus.WithError(err).Warn("GetUsersBatch failed, returning conversations without enrichment")
		profiles = map[int64]*UserInfo{}
	}

	for _, c := range convs {
		ec := &EnrichedConversation{Conversation: c}
		if info, ok := profiles[c.OtherParticipant(userID)]; ok {
			ec.OtherUserName = info.FirstName
			ec.OtherUsername = info.Username
		}
		result = append(result, ec)
	}
	return result, nil
}

// EnrichedConversation wraps a Conversation with the other participant's display name.
type EnrichedConversation struct {
	*entity.Conversation
	OtherUserName string `json:"other_user_name,omitempty"`
	OtherUsername string `json:"other_username,omitempty"`
}

func (s *ChatService) SendMessage(ctx context.Context, convID string, senderID int64, content string) (*entity.Message, error) {
	conv, err := s.convRepo.GetByID(ctx, convID)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	if !conv.HasParticipant(senderID) {
		return nil, fmt.Errorf("user %d is not a participant of conversation %s", senderID, convID)
	}

	msg := entity.NewMessage(convID, senderID, content)
	if err := s.msgRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("persist message: %w", err)
	}

	s.broadcastMessage(conv, msg)

	go func() {
		evt, err := event.New("chat.message_sent", event.MessageSentPayload{
			ConversationID: convID,
			MessageID:      msg.ID,
			SenderID:       senderID,
		})
		if err != nil {
			return
		}
		if err := s.publisher.Publish(context.Background(), "chat.message_sent", evt); err != nil {
			logrus.WithError(err).Warn("publish chat.message_sent failed")
		}
	}()

	return msg, nil
}

func (s *ChatService) GetMessages(ctx context.Context, convID string, limit, offset int) ([]*entity.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.msgRepo.GetByConversationID(ctx, convID, limit, offset)
}

func (s *ChatService) MarkAsRead(ctx context.Context, convID string, userID int64) error {
	return s.msgRepo.MarkAsRead(ctx, convID, userID)
}

// OnMatchCreated is called by the RabbitMQ subscriber when match.created arrives.
func (s *ChatService) OnMatchCreated(body []byte) error {
	var wrapper event.DomainEvent
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}
	if wrapper.Type != "match.created" {
		return nil
	}
	var payload event.MatchCreatedPayload
	if err := json.Unmarshal(wrapper.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	_, err := s.GetOrCreateConversation(context.Background(), payload.MatchID, payload.User1ID, payload.User2ID)
	if err != nil {
		return fmt.Errorf("create conversation for match %d: %w", payload.MatchID, err)
	}
	logrus.WithField("match_id", payload.MatchID).Info("conversation created for match")
	return nil
}

// ── WebSocket broadcast helpers ───────────────────────────────────────────────

type wsMessageEvent struct {
	Type    string        `json:"type"`
	Message *wsMessageDTO `json:"message"`
}

type wsMessageDTO struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	SenderID       int64  `json:"sender_id"`
	Content        string `json:"content"`
	SentAt         string `json:"sent_at"`
}

func (s *ChatService) broadcastMessage(conv *entity.Conversation, msg *entity.Message) {
	payload, err := json.Marshal(wsMessageEvent{
		Type: "new_message",
		Message: &wsMessageDTO{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			Content:        msg.Content,
			SentAt:         msg.SentAt.Format("2006-01-02T15:04:05Z"),
		},
	})
	if err != nil {
		return
	}
	s.hub.Broadcast([]int64{conv.User1ID, conv.User2ID}, payload)
}
