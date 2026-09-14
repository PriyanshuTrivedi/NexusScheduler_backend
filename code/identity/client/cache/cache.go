package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/redis/go-redis/v9"
)

//go:generate mockgen -source=cache.go -destination=../../../../gen/mocks/identity/client/cache/cache.go -package=client

const organizationsKey = "identity:organizations"

type Client interface {
	GetOrganizations(ctx context.Context) ([]entity.Organization, bool, error)
	SetOrganizations(ctx context.Context, organizations []entity.Organization) error
}

type redisClient struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) Client {
	return &redisClient{rdb: rdb}
}

func (c *redisClient) GetOrganizations(ctx context.Context) ([]entity.Organization, bool, error) {
	value, err := c.rdb.Get(ctx, organizationsKey).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cache: get organizations: %w", err)
	}

	var organizations []entity.Organization
	if err := json.Unmarshal(value, &organizations); err != nil {
		return nil, false, fmt.Errorf("cache: unmarshal organizations: %w", err)
	}

	return organizations, true, nil
}

func (c *redisClient) SetOrganizations(ctx context.Context, organizations []entity.Organization) error {
	value, err := json.Marshal(organizations)
	if err != nil {
		return fmt.Errorf("cache: marshal organizations: %w", err)
	}

	// 0 = no expiration. The cache is refreshed explicitly whenever
	// the underlying organization data changes.
	if err := c.rdb.Set(ctx, organizationsKey, value, 0).Err(); err != nil {
		return fmt.Errorf("cache: set organizations: %w", err)
	}

	return nil
}
