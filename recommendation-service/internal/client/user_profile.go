package client

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	userprofilev1 "github.com/mihnpro/DatingBotProtos/gen/go/user-profile/v1"
)

// ProfileStats holds the denormalised profile data needed by the ranking engine.
type ProfileStats struct {
	UserID          int64
	Age             int32
	Gender          string
	City            string
	Interests       []string
	PhotosCount     int32
	FullnessPercent float32
}

// UserProfileClient is a thread-safe singleton gRPC client for user-profile-service.
type UserProfileClient struct {
	conn   *grpc.ClientConn
	client userprofilev1.UserServiceClient
}

var (
	upInstance *UserProfileClient
	upOnce     sync.Once
	upInitErr  error
)

// InitUserProfileClient initialises the singleton gRPC connection.
// Safe to call multiple times; only the first call has any effect.
func InitUserProfileClient(addr string) error {
	upOnce.Do(func() {
		conn, err := grpc.NewClient(
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			upInitErr = fmt.Errorf("user-profile-service grpc dial %q: %w", addr, err)
			return
		}
		upInstance = &UserProfileClient{
			conn:   conn,
			client: userprofilev1.NewUserServiceClient(conn),
		}
	})
	return upInitErr
}

// GetUserProfileClient returns the singleton client.
// Panics if InitUserProfileClient has not been called successfully.
func GetUserProfileClient() *UserProfileClient {
	if upInstance == nil {
		panic("UserProfileClient not initialised — call InitUserProfileClient first")
	}
	return upInstance
}

// Close releases the underlying gRPC connection.
func (c *UserProfileClient) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetProfileStats fetches a user's profile from user-profile-service and maps
// it to the lightweight ProfileStats struct used by the ranking engine.
// Returns (nil, nil) when the profile does not exist yet.
func (c *UserProfileClient) GetProfileStats(ctx context.Context, userID int64) (*ProfileStats, error) {
	resp, err := c.client.GetProfile(ctx, &userprofilev1.GetProfileRequest{UserId: userID})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("GetProfileStats user_id=%d: %w", userID, err)
	}
	if resp.Profile == nil {
		return nil, nil
	}
	p := resp.Profile
	return &ProfileStats{
		UserID:          p.UserId,
		Age:             p.Age,
		Gender:          p.Gender,
		City:            p.City,
		Interests:       p.Interests,
		PhotosCount:     p.PhotosCount,
		FullnessPercent: p.FullnessPercent,
	}, nil
}

// GetProfileStatsBatch fetches multiple profiles concurrently using one
// goroutine per userID, bounded by the supplied semaphore channel size.
// Results for missing profiles are omitted from the returned map.
func (c *UserProfileClient) GetProfileStatsBatch(
	ctx context.Context,
	userIDs []int64,
	concurrency int,
) (map[int64]*ProfileStats, error) {
	if concurrency <= 0 {
		concurrency = 10
	}

	type result struct {
		stats *ProfileStats
		err   error
	}

	sem := make(chan struct{}, concurrency)
	results := make(chan result, len(userIDs))

	var wg sync.WaitGroup
	for _, id := range userIDs {
		wg.Add(1)
		go func(uid int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			stats, err := c.GetProfileStats(ctx, uid)
			results <- result{stats: stats, err: err}
		}(id)
	}

	// Close the results channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	out := make(map[int64]*ProfileStats, len(userIDs))
	for r := range results {
		if r.err != nil {
			return nil, r.err
		}
		if r.stats != nil {
			out[r.stats.UserID] = r.stats
		}
	}
	return out, nil
}

// isNotFound reports whether the gRPC error has code NotFound.
func isNotFound(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.NotFound
}
