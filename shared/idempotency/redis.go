package idempotency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Checker records whether an event has already been processed, so an
// at-least-once Kafka redelivery doesn't re-apply a saga step twice.
type Checker interface {
	// AlreadyProcessed returns true if this eventID was seen before, and
	// atomically marks it as processed if not.
	AlreadyProcessed(ctx context.Context, eventID string) (bool, error)
}

type RedisChecker struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

func NewRedisChecker(client *redis.Client, keyPrefix string) *RedisChecker {
	return &RedisChecker{client: client, keyPrefix: keyPrefix, ttl: 24 * time.Hour}
}

func (c *RedisChecker) AlreadyProcessed(ctx context.Context, eventID string) (bool, error) {
	key := c.keyPrefix + ":" + eventID
	set, err := c.client.SetNX(ctx, key, 1, c.ttl).Result()
	if err != nil {
		return false, err
	}
	return !set, nil
}
