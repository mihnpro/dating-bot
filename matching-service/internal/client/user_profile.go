package client

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userprofilev1 "github.com/mihnpro/DatingBotProtos/gen/go/user-profile/v1"
)

type UserProfileClient struct {
	conn   *grpc.ClientConn
	client userprofilev1.UserServiceClient
}

var (
	instance *UserProfileClient
	once     sync.Once
)

// Init creates the User Profile Service gRPC client connection.
func Init(addr string) error {
	var err error
	once.Do(func() {
		conn, dialErr := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if dialErr != nil {
			err = fmt.Errorf("connect to user-profile-service: %w", dialErr)
			return
		}
		instance = &UserProfileClient{
			conn:   conn,
			client: userprofilev1.NewUserServiceClient(conn),
		}
	})
	return err
}

// Get returns the singleton client.
func Get() *UserProfileClient {
	return instance
}

// ValidateUser checks if a user exists in User Profile Service.
func (c *UserProfileClient) ValidateUser(ctx context.Context, userID int64) (bool, error) {
	_, err := c.client.GetUser(ctx, &userprofilev1.GetUserRequest{Id: userID})
	if err != nil {
		// gRPC NotFound or Internal
		return false, err
	}
	return true, nil
}

// GetUserByTelegramID fetches user by telegram ID.
func (c *UserProfileClient) GetUserByTelegramID(ctx context.Context, telegramID int64) (*userprofilev1.GetUserResponse, error) {
	return c.client.GetUserByTelegramID(ctx, &userprofilev1.GetUserByTelegramIDRequest{
		TelegramId: telegramID,
	})
}

// Close closes the gRPC connection.
func (c *UserProfileClient) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
