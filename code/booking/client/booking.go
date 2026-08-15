package client

import (
	"context"

	"github.com/redis/go-redis/v9"
)

//go:generate mockgen -source=booking.go -destination=../../../gen/mocks/booking/client/booking_mock.go -package=mocks

type Client interface {
	AcquireLock(ctx context.Context, key string) (release func(), ok bool, err error)
	PublishEvent(ctx context.Context, eventType, referenceCode string) error
}

type client struct {
	redis *redis.Client
}

func New(redis *redis.Client) Client {
	return &client{redis: redis}
}

func (c *client) AcquireLock(ctx context.Context, key string) (func(), bool, error) {
	ok, err := c.redis.SetNX(ctx, "lock:"+key, "1", 0).Result()
	if err != nil || !ok {
		return nil, false, err
	}
	return func() { c.redis.Del(ctx, "lock:"+key) }, true, nil
}

func (c *client) PublishEvent(ctx context.Context, eventType, referenceCode string) error {
	// TODO: publish to booking.events stream (Redpanda)
	return nil
}
