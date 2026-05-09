package client

import (
	"context"
	"fmt"

	userprofilev1 "github.com/mihnpro/DatingBotProtos/gen/go/user-profile/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/dating-bot/chat-service/internal/service"
)

// UserProfileClient is a gRPC client for user-profile-service.
// It implements service.UserProfileFetcher.
type UserProfileClient struct {
	conn   *grpc.ClientConn
	client userprofilev1.UserServiceClient
}

func NewUserProfileClient(addr string) (*UserProfileClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial user-profile-service %q: %w", addr, err)
	}
	return &UserProfileClient{
		conn:   conn,
		client: userprofilev1.NewUserServiceClient(conn),
	}, nil
}

func (c *UserProfileClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetUser fetches basic user info and enriches with profile data. Returns nil if not found.
func (c *UserProfileClient) GetUser(ctx context.Context, userID int64) (*service.UserInfo, error) {
	resp, err := c.client.GetUser(ctx, &userprofilev1.GetUserRequest{Id: userID})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("GetUser user_id=%d: %w", userID, err)
	}
	if resp.User == nil {
		return nil, nil
	}
	u := resp.User
	info := &service.UserInfo{
		UserID:    u.Id,
		FirstName: u.FirstName,
		Username:  u.Username,
	}

	profileResp, err := c.client.GetProfile(ctx, &userprofilev1.GetProfileRequest{UserId: userID})
	if err == nil && profileResp.Profile != nil {
		p := profileResp.Profile
		info.Age = p.Age
		info.City = p.City
		info.Gender = p.Gender
	}
	return info, nil
}

// GetUsersBatch fetches multiple users concurrently. Missing users are omitted.
func (c *UserProfileClient) GetUsersBatch(ctx context.Context, userIDs []int64) (map[int64]*service.UserInfo, error) {
	type result struct {
		info *service.UserInfo
		err  error
	}

	ch := make(chan result, len(userIDs))
	for _, id := range userIDs {
		go func(uid int64) {
			info, err := c.GetUser(ctx, uid)
			ch <- result{info: info, err: err}
		}(id)
	}

	out := make(map[int64]*service.UserInfo, len(userIDs))
	for range userIDs {
		r := <-ch
		if r.err != nil {
			return nil, r.err
		}
		if r.info != nil {
			out[r.info.UserID] = r.info
		}
	}
	return out, nil
}

func isNotFound(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.NotFound
}
