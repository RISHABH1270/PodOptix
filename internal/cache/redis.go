package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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
// Startup Ping is capped at 30s so an unreachable Redis fails fast with a clean
// error instead of hanging the pod until Kubernetes' liveness probe fires.
func New(redisURL string) (*Cache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = client.Ping(ctx).Err(); err != nil {
		client.Close() // don't leak the half-open client on startup error
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

// Lua script runs atomically on the Redis server: deletes the lock key ONLY if its
// current value matches our token. Prevents a long-running job from accidentally
// deleting a NEW lock acquired by someone else after the original lock expired.
//
// KEYS[1] = lock key
// ARGV[1] = token the caller stored when acquiring the lock
// Returns 1 if deleted (we still held it), 0 if the lock was ours no more (TTL expired
// and someone else holds a fresh one, or it was already deleted).
var releaseLockScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	else
		return 0
	end
`)

func recalculateLockKey(clusterID string) string {
	return fmt.Sprintf("lock:cluster:%s:recalculate", clusterID)
}

// AcquireRecalculateLock tries to acquire a distributed lock for a cluster recalculation.
// Returns (token, true) if acquired; the caller MUST pass that token to
// ReleaseRecalculateLock. Returns ("", false) if the lock is already held.
//
// The token is a random UUID stored as the lock's value — ReleaseRecalculateLock
// compares against it so a stale caller can't delete a lock that was re-acquired
// by someone else after TTL expiry.
func (c *Cache) AcquireRecalculateLock(ctx context.Context, clusterID string) (string, bool, error) {
	token := uuid.New().String()
	ok, err := c.client.SetNX(ctx, recalculateLockKey(clusterID), token, RecalculateLockTTL).Result()
	if err != nil {
		return "", false, fmt.Errorf("acquire recalculate lock: %w", err)
	}
	if !ok {
		return "", false, nil
	}
	return token, true, nil
}

// ReleaseRecalculateLock releases the lock IFF the stored value still matches our
// token. Safe to call even if the TTL already expired and someone else owns the lock
// now — the Lua CAS turns the delete into a no-op in that case.
//
// Always called via defer on goroutine exit (success or failure).
func (c *Cache) ReleaseRecalculateLock(ctx context.Context, clusterID, token string) error {
	_, err := releaseLockScript.Run(ctx, c.client, []string{recalculateLockKey(clusterID)}, token).Result()
	// redis.Nil is "script returned nothing" — happens when the Lua returns 0 (key gone).
	// That's not an error for us; we treat it as "no-op release" per the comment above.
	if err != nil && err != redis.Nil {
		return fmt.Errorf("release recalculate lock: %w", err)
	}
	return nil
}
