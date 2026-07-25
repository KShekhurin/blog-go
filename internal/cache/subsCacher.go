package cache

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type SubsCacher interface {
	SubscribeUserTo(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error
	UnsubscribeUserFrom(ctx context.Context, sub_id uuid.UUID, auth_id uuid.UUID) error
}

type subsCacher struct {
	cache *redis.Client
}

func NewSubsCacher(client *redis.Client) SubsCacher {
	return &subsCacher{
		cache: client,
	}
}

func (c subsCacher) SubscribeUserTo(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	subsKey := fmt.Sprintf("user:%s:followers", authId)
	subscriptionKey := fmt.Sprintf("user:%s:subscriptions", subId)

	pipeline := c.cache.TxPipeline()

	pipeline.SAdd(ctx, subsKey, subId)
	pipeline.SAdd(ctx, subscriptionKey, authId)

	_, err := pipeline.Exec(ctx)

	return err
}

func (c subsCacher) UnsubscribeUserFrom(ctx context.Context, subId uuid.UUID, authId uuid.UUID) error {
	subsKey := fmt.Sprintf("user:%s:followers", authId)
	subscriptionKey := fmt.Sprintf("user:%s:subscriptions", subId)

	pipeline := c.cache.TxPipeline()

	pipeline.SRem(ctx, subsKey, subId)
	pipeline.SRem(ctx, subscriptionKey, authId)

	_, err := pipeline.Exec(ctx)

	return err
}
