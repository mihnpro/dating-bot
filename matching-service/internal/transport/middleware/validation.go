package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dating-bot/matching-service/internal/client"
)

// UserIDMethods lists methods where user IDs should be validated.
var UserIDMethods = map[string][]string{
	"/matching.v1.MatchingService/Like":                    {"from_user_id", "to_user_id"},
	"/matching.v1.MatchingService/Pass":                    {"from_user_id", "to_user_id"},
	"/matching.v1.MatchingService/UndoLike":                {"from_user_id", "to_user_id"},
	"/matching.v1.MatchingService/GetMatch":                {},
	"/matching.v1.MatchingService/GetUserMatches":          {"user_id"},
	"/matching.v1.MatchingService/HasMatched":              {"user1_id", "user2_id"},
	"/matching.v1.MatchingService/GetInteractionHistory":   {"user_id"},
	"/matching.v1.MatchingService/MarkConversationStarted": {},
}

// UserValidationInterceptor validates user IDs against User Profile Service.
func UserValidationInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		method := info.FullMethod
		fields, ok := UserIDMethods[method]
		if !ok || len(fields) == 0 {
			return handler(ctx, req)
		}

		// Extract user IDs from request using reflection-like approach
		userIDs := extractUserIDs(req, fields)

		upClient := client.Get()
		if upClient == nil {
			// Client not initialized, skip validation (dev mode)
			return handler(ctx, req)
		}

		for _, userID := range userIDs {
			if userID == 0 {
				continue
			}
			exists, err := upClient.ValidateUser(ctx, userID)
			if err != nil {
				// Check if it's a not-found error
				if status.Code(err) == codes.NotFound {
					return nil, status.Errorf(codes.NotFound, "user %d not found", userID)
				}
				// Log but don't fail on connection issues (allow graceful degradation)
				// In production, this should return an error
				return nil, status.Errorf(codes.Unavailable, "user-profile-service unavailable: %v", err)
			}
			if !exists {
				return nil, status.Errorf(codes.NotFound, "user %d not found", userID)
			}
		}

		return handler(ctx, req)
	}
}

// extractUserIDs extracts user ID values from a proto message by field name.
func extractUserIDs(req any, fields []string) []int64 {
	// Use the getter approach: check for GetXxx methods
	var ids []int64
	for _, field := range fields {
		id := getFieldAsInt64(req, field)
		ids = append(ids, id)
	}
	return ids
}

// getFieldAsInt64 uses reflection to get a field value from a proto message.
func getFieldAsInt64(req any, fieldName string) int64 {
	// Type assertions for known proto message types
	switch r := req.(type) {
	case interface{ GetFromUserId() int64 }:
		if fieldName == "from_user_id" {
			return r.GetFromUserId()
		}
	case interface{ GetToUserId() int64 }:
		if fieldName == "to_user_id" {
			return r.GetToUserId()
		}
	case interface{ GetUserId() int64 }:
		if fieldName == "user_id" {
			return r.GetUserId()
		}
	case interface{ GetUser1Id() int64 }:
		if fieldName == "user1_id" {
			return r.GetUser1Id()
		}
	case interface{ GetUser2Id() int64 }:
		if fieldName == "user2_id" {
			return r.GetUser2Id()
		}
	}
	return 0
}
