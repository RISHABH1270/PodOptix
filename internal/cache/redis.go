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
	RecommendationTTL = 1 * time.Hour

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

	var client *redis.Client
	client = redis.NewClient(opts)

	// verify connection
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

// ── Recommendations cache ────────────────────────────────────────────────────

// SetRecommendations caches recommendations for a cluster as JSON.
// TTL is 1 hour — after that the next request fetches fresh from PostgreSQL.
func (c *Cache) SetRecommendations(ctx context.Context, clusterID string, data interface{}) error {
	var key string
	key = fmt.Sprintf("cluster:%s:recommendations", clusterID)

	var bytes []byte
	var err error
	bytes, err = json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal recommendations: %w", err)
	}

	return c.client.Set(ctx, key, bytes, RecommendationTTL).Err()
}

// GetRecommendations fetches cached recommendations for a cluster.
// Returns (nil, nil) if cache miss — caller should query PostgreSQL.
func (c *Cache) GetRecommendations(ctx context.Context, clusterID string, dest interface{}) (bool, error) {
	var key string
	key = fmt.Sprintf("cluster:%s:recommendations", clusterID)

	var val string
	var err error
	val, err = c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// cache miss — not an error
		return false, nil
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
	var key string
	key = fmt.Sprintf("cluster:%s:recommendations", clusterID)
	return c.client.Del(ctx, key).Err()
}

// ── Distributed lock ─────────────────────────────────────────────────────────

// AcquireRecalculateLock tries to acquire a lock for recalculating a cluster.
// Returns true if lock acquired, false if already locked (recalculate in progress).
// Uses SET NX (set only if not exists) — atomic operation, no race conditions.
func (c *Cache) AcquireRecalculateLock(ctx context.Context, clusterID string) (bool, error) {
	var key string
	key = fmt.Sprintf("lock:cluster:%s:recalculate", clusterID)

	// SET key "1" NX EX 300 — set ONLY if key does not exist
	var ok bool
	var err error
	ok, err = c.client.SetNX(ctx, key, "1", RecalculateLockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("acquire recalculate lock: %w", err)
	}
	return ok, nil
}

// ReleaseRecalculateLock releases the recalculate lock for a cluster.
// Called when recalculation completes (success or failure).
func (c *Cache) ReleaseRecalculateLock(ctx context.Context, clusterID string) error {
	var key string
	key = fmt.Sprintf("lock:cluster:%s:recalculate", clusterID)
	return c.client.Del(ctx, key).Err()
}

// ── Job queue ────────────────────────────────────────────────────────────────

// EnqueueRecalculate adds a cluster to the recalculate job queue.
func (c *Cache) EnqueueRecalculate(ctx context.Context, clusterID string) error {
	return c.client.LPush(ctx, "recalculate-jobs", clusterID).Err()
}

// DequeueRecalculate pops the next cluster ID from the recalculate queue.
// Returns ("", nil) if queue is empty.
func (c *Cache) DequeueRecalculate(ctx context.Context) (string, error) {
	var val string
	var err error
	val, err = c.client.RPop(ctx, "recalculate-jobs").Result()
	if err == redis.Nil {
		// queue empty — not an error
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("dequeue recalculate: %w", err)
	}
	return val, nil
}
