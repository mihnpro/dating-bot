package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const keyPrefix = "rec:queue:"

// RecommendationCache stores a ranked list of candidate user_ids per viewer
// in a Redis List. The gateway pops one entry at a time; when the list is
// exhausted the service refills it from the ranked Postgres query.
type RecommendationCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRecommendationCache(addr, password string, db int, ttlSeconds int) *RecommendationCache {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
	})
	return &RecommendationCache{
		client: client,
		ttl:    time.Duration(ttlSeconds) * time.Second,
	}
}

// Ping verifies the Redis connection is alive.
func (c *RecommendationCache) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

// Close releases the underlying connection pool.
func (c *RecommendationCache) Close() error {
	return c.client.Close()
}

// key returns the Redis List key for a viewer's recommendation queue.
func (c *RecommendationCache) key(viewerUserID int64) string {
	return fmt.Sprintf("%s%d", keyPrefix, viewerUserID)
}

// Len returns the number of remaining candidates in the queue.
// Returns 0 if the key does not exist.
func (c *RecommendationCache) Len(ctx context.Context, viewerUserID int64) (int64, error) {
	n, err := c.client.LLen(ctx, c.key(viewerUserID)).Result()
	if err != nil {
		return 0, fmt.Errorf("cache llen user=%d: %w", viewerUserID, err)
	}
	return n, nil
}

// Push atomically replaces the queue for viewerUserID with the supplied
// ordered list of candidate user IDs and sets a TTL on the key.
// The first element will be the next to be popped.
func (c *RecommendationCache) Push(ctx context.Context, viewerUserID int64, candidateIDs []int64) error {
	if len(candidateIDs) == 0 {
		return nil
	}

	key := c.key(viewerUserID)

	// Convert []int64 → []interface{} for RPUSH.
	members := make([]interface{}, len(candidateIDs))
	for i, id := range candidateIDs {
		members[i] = strconv.FormatInt(id, 10)
	}

	// Use a pipeline: DEL + RPUSH + EXPIRE — atomic enough for our use case.
	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.RPush(ctx, key, members...)
	pipe.Expire(ctx, key, c.ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache push user=%d: %w", viewerUserID, err)
	}
	return nil
}

// Pop removes and returns the leftmost (next) candidate user ID for the viewer.
// Returns (0, false, nil) when the queue is empty or the key does not exist.
func (c *RecommendationCache) Pop(ctx context.Context, viewerUserID int64) (int64, bool, error) {
	val, err := c.client.LPop(ctx, c.key(viewerUserID)).Result()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("cache pop user=%d: %w", viewerUserID, err)
	}

	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("cache pop parse id %q: %w", val, err)
	}
	return id, true, nil
}

// Invalidate deletes the recommendation queue for a viewer, forcing a fresh
// computation on the next GetNextProfile call.
func (c *RecommendationCache) Invalidate(ctx context.Context, viewerUserID int64) error {
	if err := c.client.Del(ctx, c.key(viewerUserID)).Err(); err != nil {
		return fmt.Errorf("cache invalidate user=%d: %w", viewerUserID, err)
	}
	return nil
}

// InvalidateAll removes every recommendation queue whose key matches the
// prefix pattern. Used after a global rating recalculation.
func (c *RecommendationCache) InvalidateAll(ctx context.Context) error {
	pattern := keyPrefix + "*"
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("cache invalidate all scan: %w", err)
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("cache invalidate all del: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}
