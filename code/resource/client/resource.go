// Package client holds outbound calls only — for Resource, that's Redis.
// Redis here is strictly a latency optimization for SearchResources; nothing
// in this package is ever the source of truth for a slot's status, mirroring
// the same "Redis as optimization, Postgres as correctness" split documented
// for Booking's locking use of Redis (Section 6.1).
package client

//go:generate mockgen -source=resource.go -destination=../../../gen/mocks/resource/client/resource.go -package=client

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client is the interface controller depends on.
type Client interface {
	// GetCachedSearch returns a previously cached SearchResources response
	// body (already proto-marshaled by the caller), or ok=false on a miss.
	GetCachedSearch(ctx context.Context, key string) (value []byte, ok bool, err error)
	SetCachedSearch(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// InvalidateOrgSearchCache bumps a per-org cache-key version so every
	// previously cached search result for that org is implicitly stale on
	// the very next read, without needing to enumerate and delete each key.
	// Called after any resource mutation (create, recurrence change, slot
	// exception, leave period) that could change a search result.
	InvalidateOrgSearchCache(ctx context.Context, orgID string) error
	// SearchCacheVersion returns the current cache-key version for an org,
	// used by the controller to build the cache key it reads/writes.
	SearchCacheVersion(ctx context.Context, orgID string) (int64, error)
	InvalidateGlobalSearchCache(ctx context.Context) error
	GlobalSearchCacheVersion(ctx context.Context) (int64, error)
}

type redisClient struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) Client {
	return &redisClient{rdb: rdb}
}

func versionKey(orgID string) string {
	return fmt.Sprintf("resource:search:version:%s", orgID)
}

const globalVersionKey = "resource:search:global-version"

func (c *redisClient) SearchCacheVersion(ctx context.Context, orgID string) (int64, error) {
	v, err := c.rdb.Get(ctx, versionKey(orgID)).Int64()
	if err == redis.Nil {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("client: get search cache version: %w", err)
	}
	return v, nil
}

func (c *redisClient) InvalidateOrgSearchCache(ctx context.Context, orgID string) error {
	if err := c.rdb.Incr(ctx, versionKey(orgID)).Err(); err != nil {
		return fmt.Errorf("client: bump search cache version: %w", err)
	}
	return nil
}

func (c *redisClient) InvalidateGlobalSearchCache(ctx context.Context) error {
	if err := c.rdb.Incr(ctx, globalVersionKey).Err(); err != nil {
		return fmt.Errorf("client: bump global search cache version: %w", err)
	}
	return nil
}

func (c *redisClient) GlobalSearchCacheVersion(ctx context.Context) (int64, error) {
	v, err := c.rdb.Get(ctx, globalVersionKey).Int64()
	if err == redis.Nil {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("client: get global search cache version: %w", err)
	}
	return v, nil
}
func (c *redisClient) GetCachedSearch(ctx context.Context, key string) ([]byte, bool, error) {
	v, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("client: get cached search: %w", err)
	}
	return v, true, nil
}

func (c *redisClient) SetCachedSearch(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("client: set cached search: %w", err)
	}
	return nil
}
