package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// RecommendationTTL — how long recommendations are cached per cluster
	RecommendationTTL = 3 * time.Hour

	// RecalculateLockTTL — how long a recalculate lock is held per cluster
	RecalculateLockTTL = 10 * time.Minute
)

// Cache wraps the Redis client and provides domain-specific methods.
type Cache struct {
	client *redis.Client
}

// New connects to Redis and returns a Cache.
func New(redisURL string) (*Cache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	if err = client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Cache{client: client}, nil
}

// Close shuts down the Redis connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

// Ping verifies the Redis connection is alive — used by readiness probe.
func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// FlushDB removes all keys in the current Redis database — used in tests to ensure clean state.
func (c *Cache) FlushDB(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}

// ── Recommendations cache ────────────────────────────────────────────────────

// SetRecommendations caches recommendations for a cluster as JSON with a 3 hour TTL.
func (c *Cache) SetRecommendations(ctx context.Context, clusterID string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal recommendations: %w", err)
	}
	return c.client.Set(ctx, fmt.Sprintf("cluster:%s:recommendations", clusterID), b, RecommendationTTL).Err()
}

// GetRecommendations fetches cached recommendations for a cluster.
// Returns (false, nil) on cache miss — caller should query PostgreSQL.
func (c *Cache) GetRecommendations(ctx context.Context, clusterID string, dest interface{}) (bool, error) {
	val, err := c.client.Get(ctx, fmt.Sprintf("cluster:%s:recommendations", clusterID)).Result()
	if err == redis.Nil {
		return false, nil // cache miss — not an error
	}
	if err != nil {
		return false, fmt.Errorf("get recommendations from cache: %w", err)
	}
	if err = json.Unmarshal([]byte(val), dest); err != nil {
		return false, fmt.Errorf("unmarshal recommendations: %w", err)
	}
	return true, nil
}

// InvalidateRecommendations removes cached recommendations for a cluster.
// Called after scheduler run or manual recalculate completes.
func (c *Cache) InvalidateRecommendations(ctx context.Context, clusterID string) error {
	return c.client.Del(ctx, fmt.Sprintf("cluster:%s:recommendations", clusterID)).Err()
}

// ── Distributed lock ─────────────────────────────────────────────────────────

// AcquireRecalculateLock tries to acquire a distributed lock for a cluster recalculation.
// Returns true if lock acquired, false if already locked (recalculation in progress).
// Uses SETNX — atomic, no race conditions.
func (c *Cache) AcquireRecalculateLock(ctx context.Context, clusterID string) (bool, error) {
	ok, err := c.client.SetNX(ctx, fmt.Sprintf("lock:cluster:%s:recalculate", clusterID), "1", RecalculateLockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("acquire recalculate lock: %w", err)
	}
	return ok, nil
}

// ReleaseRecalculateLock releases the recalculate lock for a cluster.
// Always called on goroutine exit — success or failure.
func (c *Cache) ReleaseRecalculateLock(ctx context.Context, clusterID string) error {
	return c.client.Del(ctx, fmt.Sprintf("lock:cluster:%s:recalculate", clusterID)).Err()
}
