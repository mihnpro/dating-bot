package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	userprofilev1 "github.com/mihnpro/DatingBotProtos/gen/go/user-profile/v1"
)

// UserProfileClient is a gRPC client for user-profile-service.
// Used to resolve internal user_id → Telegram telegram_id before delivery.
type UserProfileClient struct {
	conn   *grpc.ClientConn
	client userprofilev1.UserServiceClient
}

func NewUserProfileClient(addr string) (*UserProfileClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("user-profile-service grpc dial %q: %w", addr, err)
	}
	return &UserProfileClient{
		conn:   conn,
		client: userprofilev1.NewUserServiceClient(conn),
	}, nil
}

func (c *UserProfileClient) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetTelegramID returns the Telegram ID for the given internal user_id.
// Returns (0, nil) when the user does not exist.
func (c *UserProfileClient) GetTelegramID(ctx context.Context, userID int64) (int64, error) {
	resp, err := c.client.GetUser(ctx, &userprofilev1.GetUserRequest{Id: userID})
	if err != nil {
		if isNotFound(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("GetUser user_id=%d: %w", userID, err)
	}
	if resp.User == nil {
		return 0, nil
	}
	return resp.User.TelegramId, nil
}

func isNotFound(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.NotFound
}
