package client

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	matchingv1 "github.com/mihnpro/DatingBotProtos/gen/go/matching/v1"
)

// MatchingClient is a thin gRPC wrapper around matching-service.
// It is used by the ranking engine to pull interaction statistics
// (likes received, passes received, match count) for behavioral scoring.
type MatchingClient struct {
	conn   *grpc.ClientConn
	client matchingv1.MatchingServiceClient
}

var (
	matchingInstance *MatchingClient
	matchingOnce     sync.Once
)

// InitMatching dials matching-service and stores the singleton client.
func InitMatching(addr string) error {
	var dialErr error
	matchingOnce.Do(func() {
		conn, err := grpc.NewClient(
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			dialErr = fmt.Errorf("dial matching-service at %s: %w", addr, err)
			return
		}
		matchingInstance = &MatchingClient{
			conn:   conn,
			client: matchingv1.NewMatchingServiceClient(conn),
		}
	})
	return dialErr
}

// GetMatching returns the singleton client.
// Panics if InitMatching was not called first.
func GetMatching() *MatchingClient {
	if matchingInstance == nil {
		panic("matching client not initialised — call InitMatching first")
	}
	return matchingInstance
}

// Close releases the underlying gRPC connection.
func (c *MatchingClient) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// InteractionStats holds the raw counts needed to compute the behavioral rating.
type InteractionStats struct {
	LikesReceived  int64
	PassesReceived int64
	MatchCount     int64
}

// GetInteractionStats fetches like/pass history for targetUserID and counts
// how many times they were liked, passed, and matched.
//
// It performs two concurrent gRPC calls:
//  1. GetInteractionHistory — to count likes/passes directed AT the user.
//  2. GetUserMatches        — to count active matches.
//
// Both calls share the same context so a deadline propagates to both.
func (c *MatchingClient) GetInteractionStats(
	ctx context.Context,
	targetUserID int64,
) (*InteractionStats, error) {
	type histResult struct {
		likes  int64
		passes int64
		err    error
	}
	type matchResult struct {
		count int64
		err   error
	}

	histCh := make(chan histResult, 1)
	matchCh := make(chan matchResult, 1)

	// Goroutine 1: fetch interaction history directed at targetUserID.
	go func() {
		resp, err := c.client.GetInteractionHistory(ctx, &matchingv1.GetInteractionHistoryRequest{
			UserId:   targetUserID,
			Page:     1,
			PageSize: 1000, // large page — we only need the total counts
		})
		if err != nil {
			histCh <- histResult{err: err}
			return
		}

		var likes, passes int64
		for _, interaction := range resp.Interactions {
			if interaction.ToUserId != targetUserID {
				continue
			}
			switch interaction.Type {
			case "like":
				likes++
			case "pass":
				passes++
			}
		}
		histCh <- histResult{likes: likes, passes: passes}
	}()

	// Goroutine 2: fetch the user's match list to count active matches.
	go func() {
		resp, err := c.client.GetUserMatches(ctx, &matchingv1.GetUserMatchesRequest{
			UserId:   targetUserID,
			Page:     1,
			PageSize: 1000,
		})
		if err != nil {
			matchCh <- matchResult{err: err}
			return
		}
		matchCh <- matchResult{count: int64(resp.TotalCount)}
	}()

	// Collect both results — whichever finishes first is already buffered.
	hist := <-histCh
	if hist.err != nil {
		return nil, fmt.Errorf("get interaction history for user %d: %w", targetUserID, hist.err)
	}

	matches := <-matchCh
	if matches.err != nil {
		return nil, fmt.Errorf("get user matches for user %d: %w", targetUserID, matches.err)
	}

	return &InteractionStats{
		LikesReceived:  hist.likes,
		PassesReceived: hist.passes,
		MatchCount:     matches.count,
	}, nil
}

// GetUserMatches returns match IDs for a user — used to build the exclusion
// list when filling a viewer's recommendation queue.
func (c *MatchingClient) GetUserMatches(
	ctx context.Context,
	userID int64,
) (*matchingv1.GetUserMatchesResponse, error) {
	return c.client.GetUserMatches(ctx, &matchingv1.GetUserMatchesRequest{
		UserId:   userID,
		Page:     1,
		PageSize: 1000,
	})
}

// GetInteractionHistory returns the raw interaction history for a user.
// Used to build the exclusion list (already-seen profiles) for the feed.
func (c *MatchingClient) GetInteractionHistory(
	ctx context.Context,
	userID int64,
) (*matchingv1.GetInteractionHistoryResponse, error) {
	return c.client.GetInteractionHistory(ctx, &matchingv1.GetInteractionHistoryRequest{
		UserId:   userID,
		Page:     1,
		PageSize: 2000,
	})
}
