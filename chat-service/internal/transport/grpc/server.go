package grpc

import (
	"context"

	chatv1 "github.com/mihnpro/DatingBotProtos/gen/go/chat/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dating-bot/chat-service/internal/domain/entity"
	"github.com/dating-bot/chat-service/internal/service"
)

// Server implements the gRPC ChatServiceServer interface.
type Server struct {
	chatv1.UnimplementedChatServiceServer
	chatSvc  *service.ChatService
	tokenSvc *service.TokenService
}

func NewServer(chatSvc *service.ChatService, tokenSvc *service.TokenService) *Server {
	return &Server{chatSvc: chatSvc, tokenSvc: tokenSvc}
}

func (s *Server) GetOrCreateConversation(
	ctx context.Context, req *chatv1.GetOrCreateConversationRequest,
) (*chatv1.ConversationResponse, error) {
	conv, err := s.chatSvc.GetOrCreateConversation(ctx, req.MatchId, req.User1Id, req.User2Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get or create conversation: %v", err)
	}
	return &chatv1.ConversationResponse{Conversation: domainToProtoConv(conv)}, nil
}

func (s *Server) GetConversation(
	ctx context.Context, req *chatv1.GetConversationRequest,
) (*chatv1.ConversationResponse, error) {
	conv, err := s.chatSvc.GetConversation(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "conversation not found: %v", err)
	}
	return &chatv1.ConversationResponse{Conversation: domainToProtoConv(conv)}, nil
}

func (s *Server) GetUserConversations(
	ctx context.Context, req *chatv1.GetUserConversationsRequest,
) (*chatv1.GetUserConversationsResponse, error) {
	convs, err := s.chatSvc.GetUserConversations(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list conversations: %v", err)
	}
	protoConvs := make([]*chatv1.Conversation, 0, len(convs))
	for _, c := range convs {
		protoConvs = append(protoConvs, domainToProtoConv(c))
	}
	return &chatv1.GetUserConversationsResponse{Conversations: protoConvs}, nil
}

func (s *Server) GetMessages(
	ctx context.Context, req *chatv1.GetMessagesRequest,
) (*chatv1.GetMessagesResponse, error) {
	limit := int(req.Limit)
	offset := int(req.Offset)
	msgs, err := s.chatSvc.GetMessages(ctx, req.ConversationId, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get messages: %v", err)
	}
	protoMsgs := make([]*chatv1.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		protoMsgs = append(protoMsgs, domainToProtoMsg(m))
	}
	return &chatv1.GetMessagesResponse{Messages: protoMsgs}, nil
}

func (s *Server) MarkAsRead(
	ctx context.Context, req *chatv1.MarkAsReadRequest,
) (*emptypb.Empty, error) {
	if err := s.chatSvc.MarkAsRead(ctx, req.ConversationId, req.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "mark as read: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GenerateChatToken(
	ctx context.Context, req *chatv1.GenerateChatTokenRequest,
) (*chatv1.GenerateChatTokenResponse, error) {
	token := s.tokenSvc.Generate(req.UserId)
	return &chatv1.GenerateChatTokenResponse{
		Token:  token,
		UserId: req.UserId,
	}, nil
}

// ── domain → proto mappers ───────────────────────────────────────────────────

func domainToProtoConv(c *entity.Conversation) *chatv1.Conversation {
	return &chatv1.Conversation{
		Id:        c.ID,
		MatchId:   c.MatchID,
		User1Id:   c.User1ID,
		User2Id:   c.User2ID,
		Status:    string(c.Status),
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
}

func domainToProtoMsg(m *entity.Message) *chatv1.ChatMessage {
	return &chatv1.ChatMessage{
		Id:             m.ID,
		ConversationId: m.ConversationID,
		SenderId:       m.SenderID,
		Content:        m.Content,
		ContentType:    string(m.ContentType),
		SentAt:         timestamppb.New(m.SentAt),
		IsRead:         m.IsRead,
	}
}
